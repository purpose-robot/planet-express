package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditriverx "github.com/purpose-robot/planet-express/internal/audit/riverx"
	"github.com/purpose-robot/planet-express/internal/auth"
	"github.com/purpose-robot/planet-express/internal/krypto"
	"github.com/riverqueue/river"
)

func runInTx(ctx context.Context, dbPool *pgxpool.Pool, fn func(tx pgx.Tx) error) (err error) {
	tx, err := dbPool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()

		rollbackErr := tx.Rollback(rollbackCtx)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = fmt.Errorf("failed to rollback database changes; got: %w; %w", err, rollbackErr)
		}
	}()

	err = fn(tx)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

type Store struct {
	dbPool      *pgxpool.Pool
	encryptor   *krypto.Encryptor
	blindIndex  krypto.Key
	riverClient *river.Client[pgx.Tx]
}

func NewStore(dbPool *pgxpool.Pool, encryptor *krypto.Encryptor, blindIndex krypto.Key, riverClient *river.Client[pgx.Tx]) *Store {
	return &Store{
		dbPool:      dbPool,
		encryptor:   encryptor,
		blindIndex:  blindIndex,
		riverClient: riverClient,
	}
}

func (s *Store) Transact(ctx context.Context, txFunc func(adapters auth.Adapters) error) error {
	return runInTx(ctx, s.dbPool, func(tx pgx.Tx) error {
		adapters := auth.Adapters{
			AuditLogRepository: auditriverx.NewAuditLogJobRepository(tx, s.riverClient),

			UserRepository:       NewUserRepository(tx, s.encryptor, s.blindIndex),
			EmailJobRepository:   NewEmailJobRepository(tx, s.riverClient),
			PermissionRepository: NewPermissionRepository(tx),
		}

		return txFunc(adapters)
	})
}
