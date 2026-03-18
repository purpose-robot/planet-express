package postgres

import (
	"context"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/purpose-robot/planet-express/internal/auth"
	"github.com/purpose-robot/planet-express/internal/krypto"
)

type dbPool interface {
	Exec(ctx context.Context, stmt string, namedArgs ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, stmt string, namedArgs ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, stmt string, namedArgs ...any) pgx.Row
	SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults
}

type UserRepository struct {
	dbPool     dbPool
	encryptor  *krypto.Encryptor
	blindIndex krypto.Key
}

func NewUserRepository(dbPool dbPool, encryptor *krypto.Encryptor, blindIndex krypto.Key) *UserRepository {
	return &UserRepository{
		dbPool:     dbPool,
		encryptor:  encryptor,
		blindIndex: blindIndex,
	}
}

// computeBlindIndex computes a new blind index from the given input.
// The salt is omitted in order to produce deterministic blind indexes.
func (r *UserRepository) computeBlindIndex(message string) (string, error) {
	blindIndex, err := krypto.HashArgon2WithKey(message, r.blindIndex)
	if err != nil {
		return "", err
	}

	blindIndex.Salt = nil

	return blindIndex.String(), nil
}

func (r *UserRepository) scanUser(row pgx.Row, extra ...any) (*auth.User, error) {
	var (
		id             uuid.UUID
		createdAt      time.Time
		updatedAt      time.Time
		activated      bool
		encryptedEmail []byte
		hashedPassword krypto.Argon2Hash
	)

	destination := []any{
		&id,
		&createdAt,
		&updatedAt,
		&activated,
		&encryptedEmail,
		&hashedPassword,
	}

	err := row.Scan(append(destination, extra...)...)
	if err != nil {
		return nil, mapRepositoryError(err)
	}

	decryptedEmail, err := r.encryptor.Decrypt(encryptedEmail)
	if err != nil {
		return nil, err
	}

	return auth.UnmarshalUser(id, createdAt, updatedAt, activated, string(decryptedEmail), hashedPassword), nil
}

func (r *UserRepository) InsertToken(ctx context.Context, token *auth.Token) error {
	stmt := `
		INSERT INTO plnx_tokens (
			id, hash, scope, user_id, expiry
		) VALUES (
			@id, @hash, @scope, @user_id, @expiry
		)`

	namedArgs := pgx.NamedArgs{
		"id":      token.ID(),
		"hash":    token.Hash(),
		"scope":   token.Scope(),
		"expiry":  token.Expiry(),
		"user_id": token.UserID(),
	}

	_, err := r.dbPool.Exec(ctx, stmt, namedArgs)
	return mapRepositoryError(err)
}

func (r *UserRepository) InsertUserAndToken(ctx context.Context, user *auth.User) error {
	emailBlindIndex, err := r.computeBlindIndex(user.Email())
	if err != nil {
		return err
	}

	encryptedEmail, err := r.encryptor.Encrypt([]byte(user.Email()))
	if err != nil {
		return err
	}

	stmt := `
		INSERT INTO plnx_users (
			id, created_at, updated_at, activated, encrypted_email, hashed_password, email_blind_index
		) VALUES (
			@id, @created_at, @updated_at, @activated, @encrypted_email, @hashed_password, @email_blind_index
		)`

	namedArgs := pgx.NamedArgs{
		"id":                user.ID(),
		"created_at":        user.CreatedAt(),
		"updated_at":        user.UpdatedAt(),
		"activated":         user.Activated(),
		"encrypted_email":   encryptedEmail,
		"hashed_password":   new(user.HashedPassword()),
		"email_blind_index": emailBlindIndex,
	}

	batch := new(pgx.Batch)
	batch.Queue(stmt, namedArgs)

	if token := user.Token(); token != nil {
		stmt := `
			INSERT INTO plnx_tokens (
				id, hash, scope, user_id, expiry
			) VALUES (
				@id, @hash, @scope, @user_id, @expiry
			)`

		namedArgs := pgx.NamedArgs{
			"id":      token.ID(),
			"hash":    token.Hash(),
			"scope":   token.Scope(),
			"expiry":  token.Expiry(),
			"user_id": token.UserID(),
		}

		batch.Queue(stmt, namedArgs)
	}

	results := r.dbPool.SendBatch(ctx, batch)

	for range batch.Len() {
		if _, execErr := results.Exec(); execErr != nil {
			_ = results.Close()
			return mapRepositoryError(execErr)
		}
	}

	return mapRepositoryError(results.Close())
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*auth.User, error) {
	emailBlindIndex, err := r.computeBlindIndex(email)
	if err != nil {
		return nil, err
	}

	stmt := `
		SELECT id, created_at, updated_at, activated, encrypted_email, hashed_password
		FROM plnx_users
		WHERE email_blind_index = @email_blind_index`

	return r.scanUser(r.dbPool.QueryRow(ctx, stmt, pgx.NamedArgs{"email_blind_index": emailBlindIndex}))
}

func (r *UserRepository) GetForToken(ctx context.Context, scope auth.Scope, hash auth.Hash) (*auth.User, error) {
	stmt := `
		SELECT
			plnx_users.id, plnx_users.created_at, plnx_users.updated_at, plnx_users.activated,
			plnx_users.encrypted_email, plnx_users.hashed_password
		FROM plnx_users
		INNER JOIN plnx_tokens
			ON plnx_users.id = plnx_tokens.user_id
		WHERE plnx_tokens.hash = @hash
			AND plnx_tokens.scope = @scope
			AND plnx_tokens.expiry > @expiry`

	return r.scanUser(r.dbPool.QueryRow(ctx, stmt, pgx.NamedArgs{"hash": hash, "scope": scope, "expiry": time.Now().UTC()}))
}

func (r *UserRepository) DeleteAllForUser(ctx context.Context, scope auth.Scope, userID uuid.UUID) error {
	stmt := `DELETE FROM plnx_tokens WHERE scope = @scope AND user_id = @user_id`

	_, err := r.dbPool.Exec(ctx, stmt, pgx.NamedArgs{"scope": scope, "user_id": userID})
	return mapRepositoryError(err)
}

func (r *UserRepository) UpdateByID(ctx context.Context, id uuid.UUID, updateFn func(user *auth.User) error) error {
	selectStmt := `
		SELECT id, created_at, updated_at, activated, encrypted_email, hashed_password, email_blind_index
		FROM plnx_users
		WHERE id = @id FOR UPDATE`

	var emailBlindIndex string

	user, err := r.scanUser(r.dbPool.QueryRow(ctx, selectStmt, pgx.NamedArgs{"id": id}), &emailBlindIndex)
	if err != nil {
		return err
	}

	currentEmail := user.Email()

	err = updateFn(user)
	if err != nil {
		return err
	}

	if user.Email() != currentEmail {
		emailBlindIndex, err = r.computeBlindIndex(user.Email())
		if err != nil {
			return err
		}
	}

	encryptedEmail, err := r.encryptor.Encrypt([]byte(user.Email()))
	if err != nil {
		return err
	}

	updateStmt := `
		UPDATE plnx_users
		SET	updated_at = @updated_at, activated = @activated, encrypted_email = @encrypted_email, hashed_password = @hashed_password, email_blind_index = @email_blind_index
		WHERE id = @id`

	namedArgs := pgx.NamedArgs{
		"id":                user.ID(),
		"updated_at":        user.UpdatedAt(),
		"activated":         user.Activated(),
		"encrypted_email":   encryptedEmail,
		"hashed_password":   new(user.HashedPassword()),
		"email_blind_index": emailBlindIndex,
	}

	_, err = r.dbPool.Exec(ctx, updateStmt, namedArgs)
	return mapRepositoryError(err)
}

type PermissionRepository struct {
	dbPool dbPool
}

func NewPermissionRepository(dbPool dbPool) *PermissionRepository {
	return &PermissionRepository{
		dbPool: dbPool,
	}
}

func (r *PermissionRepository) AddForUser(ctx context.Context, id uuid.UUID, code ...string) error {
	stmt := `
		INSERT INTO plnx_users_permissions SELECT @id, plnx_permissions.id
	    FROM plnx_permissions
	    WHERE plnx_permissions.code = ANY(@code)`

	_, err := r.dbPool.Exec(ctx, stmt, pgx.NamedArgs{"id": id, "code": code})
	return mapRepositoryError(err)
}

func (r *PermissionRepository) GetAllForUser(ctx context.Context, userID uuid.UUID) (auth.Permissions, error) {
	stmt := `
	    SELECT plnx_permissions.code
	    FROM plnx_permissions
	    INNER JOIN plnx_users_permissions ON plnx_users_permissions.permission_id = plnx_permissions.id
	    INNER JOIN plnx_users ON plnx_users_permissions.user_id = plnx_users.id
	    WHERE plnx_users.id = @user_id`

	rows, err := r.dbPool.Query(ctx, stmt, pgx.NamedArgs{"user_id": userID})
	if err != nil {
		return nil, mapRepositoryError(err)
	}

	permissions, err := pgx.CollectRows(rows, pgx.RowTo[auth.Permission])
	return permissions, mapRepositoryError(err)
}
