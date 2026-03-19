package postgres

import (
	"context"
	"net/netip"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/purpose-robot/planet-express/internal/bessie"
	"github.com/purpose-robot/planet-express/internal/krypto"
)

type dbPool interface {
	Exec(ctx context.Context, stmt string, namedArgs ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, stmt string, namedArgs ...any) pgx.Row
}

type CredentialRepository struct {
	dbPool     dbPool
	encryptor  *krypto.Encryptor
	blindIndex krypto.Key
}

func NewCredentialRepository(dbPool dbPool, encryptor *krypto.Encryptor, blindIndex krypto.Key) *CredentialRepository {
	return &CredentialRepository{
		dbPool:     dbPool,
		encryptor:  encryptor,
		blindIndex: blindIndex,
	}
}

func (r *CredentialRepository) scanCredential(row pgx.Row) (*bessie.Credential, error) {
	var (
		id                uuid.UUID
		createdAt         time.Time
		updatedAt         time.Time
		userID            *uuid.UUID
		username          string
		encryptedPassword []byte
		authMethod        bessie.AuthMethod
		description       *string
		lastUsedAt        *time.Time
	)

	destination := []any{
		&id,
		&createdAt,
		&updatedAt,
		&userID,
		&username,
		&encryptedPassword,
		&authMethod,
		&description,
		&lastUsedAt,
	}

	err := row.Scan(destination...)
	if err != nil {
		return nil, mapRepositoryError(err)
	}

	decryptedPassword, err := r.encryptor.Decrypt(encryptedPassword)
	if err != nil {
		return nil, err
	}

	return bessie.UnmarshalCredential(
		id, createdAt, updatedAt, userID, username, string(decryptedPassword), authMethod, description, lastUsedAt,
	), nil
}

func (r *CredentialRepository) Insert(ctx context.Context, credentials *bessie.Credential) error {
	encryptedPassword, err := r.encryptor.Encrypt([]byte(credentials.Password()))
	if err != nil {
		return err
	}

	stmt := `
		INSERT INTO plnx_credentials (
			id, created_at, updated_at, user_id, username, encrypted_password, auth_method, description, last_used_at
		) VALUES (
			@id, @created_at, @updated_at, @user_id, @username, @encrypted_password, @auth_method, @description, @last_used_at
		)`

	namedArgs := pgx.NamedArgs{
		"id":                 credentials.ID(),
		"created_at":         credentials.CreatedAt(),
		"updated_at":         credentials.UpdatedAt(),
		"user_id":            credentials.UserID(),
		"username":           credentials.Username(),
		"encrypted_password": encryptedPassword,
		"auth_method":        credentials.AuthMethod(),
		"description":        credentials.Description(),
		"last_used_at":       credentials.LastUsedAt(),
	}

	_, err = r.dbPool.Exec(ctx, stmt, namedArgs)
	return mapRepositoryError(err)
}

func (r *CredentialRepository) GetByID(ctx context.Context, id uuid.UUID) (*bessie.Credential, error) {
	stmt := `
		SELECT id, created_at, updated_at, user_id, username, encrypted_password, auth_method, description, last_used_at
		FROM plnx_credentials
		WHERE id = @id`

	return r.scanCredential(r.dbPool.QueryRow(ctx, stmt, pgx.NamedArgs{"id": id}))
}

type NetworkDeviceRepository struct {
	dbPool dbPool
}

func NewNetworkDeviceRepository(dbPool dbPool) *NetworkDeviceRepository {
	return &NetworkDeviceRepository{
		dbPool: dbPool,
	}
}

func (r *NetworkDeviceRepository) scanNetworkDevice(row pgx.Row) (*bessie.NetworkDevice, error) {
	var (
		id                  uuid.UUID
		createdAt           time.Time
		updatedAt           time.Time
		ipAddress           netip.Addr
		hostname            *string
		serialNumber        *string
		productID           *string
		softwareVersion     *string
		lastSyncStatus      bessie.SyncStatus
		lastSyncReachable   *bool
		lastSyncAttemptedAt *time.Time
	)

	destination := []any{
		&id,
		&createdAt,
		&updatedAt,
		&ipAddress,
		&hostname,
		&serialNumber,
		&productID,
		&softwareVersion,
		&lastSyncStatus,
		&lastSyncReachable,
		&lastSyncAttemptedAt,
	}

	err := row.Scan(destination...)
	if err != nil {
		return nil, mapRepositoryError(err)
	}

	return bessie.UnmarshalNetworkDevice(id, createdAt, updatedAt, ipAddress, hostname, serialNumber, productID, softwareVersion, lastSyncStatus, lastSyncReachable, lastSyncAttemptedAt), nil
}

func (r *NetworkDeviceRepository) Insert(ctx context.Context, networkDevice *bessie.NetworkDevice) (bool, error) {
	stmt := `
		INSERT INTO plnx_network_devices (
			id, created_at, updated_at, ip_address, last_sync_status
		) VALUES (
			@id, @created_at, @updated_at, @ip_address, @last_sync_status
		) ON CONFLICT (ip_address) DO NOTHING`

	namedArgs := pgx.NamedArgs{
		"id":               networkDevice.ID(),
		"created_at":       networkDevice.CreatedAt(),
		"updated_at":       networkDevice.UpdatedAt(),
		"ip_address":       networkDevice.IPAddress(),
		"last_sync_status": networkDevice.LastSyncStatus(),
	}

	tag, err := r.dbPool.Exec(ctx, stmt, namedArgs)
	return tag.RowsAffected() == 1, mapRepositoryError(err)
}

func (r *NetworkDeviceRepository) GetByID(ctx context.Context, id uuid.UUID) (*bessie.NetworkDevice, error) {
	stmt := `
		SELECT id, created_at, updated_at, ip_address, hostname, serial_number, product_id, software_version, last_sync_status, last_sync_reachable, last_sync_attempted_at
		FROM plnx_network_devices
		WHERE id = @id`

	return r.scanNetworkDevice(r.dbPool.QueryRow(ctx, stmt, pgx.NamedArgs{"id": id}))
}

func (r *NetworkDeviceRepository) UpdateByID(ctx context.Context, id uuid.UUID, updateFn func(nd *bessie.NetworkDevice) error) error {
	selectStmt := `
		SELECT id, created_at, updated_at, ip_address, hostname, serial_number, product_id, software_version, last_sync_status, last_sync_reachable, last_sync_attempted_at
		FROM plnx_network_devices
		WHERE id = @id FOR UPDATE`

	networkDevice, err := r.scanNetworkDevice(r.dbPool.QueryRow(ctx, selectStmt, pgx.NamedArgs{"id": id}))
	if err != nil {
		return err
	}

	err = updateFn(networkDevice)
	if err != nil {
		return err
	}

	updateStmt := `
		UPDATE plnx_network_devices
		SET
			updated_at = @updated_at,
			ip_address = @ip_address,
			hostname = @hostname,
			serial_number = @serial_number,
			product_id = @product_id,
			software_version = @software_version,
			last_sync_status = @last_sync_status,
			last_sync_reachable = @last_sync_reachable,
			last_sync_attempted_at = @last_sync_attempted_at
		WHERE id = @id`

	namedArgs := pgx.NamedArgs{
		"id":                     networkDevice.ID(),
		"updated_at":             networkDevice.UpdatedAt(),
		"ip_address":             networkDevice.IPAddress(),
		"hostname":               networkDevice.Hostname(),
		"serial_number":          networkDevice.SerialNumber(),
		"product_id":             networkDevice.ProductID(),
		"software_version":       networkDevice.SoftwareVersion(),
		"last_sync_status":       networkDevice.LastSyncStatus(),
		"last_sync_reachable":    networkDevice.LastSyncReachable(),
		"last_sync_attempted_at": networkDevice.LastSyncAttemptedAt(),
	}

	_, err = r.dbPool.Exec(ctx, updateStmt, namedArgs)
	return mapRepositoryError(err)
}
