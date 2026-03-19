package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditriverx "github.com/purpose-robot/planet-express/internal/audit/riverx"
	"github.com/purpose-robot/planet-express/internal/cisco"
	"github.com/purpose-robot/planet-express/internal/krypto"
	"github.com/riverqueue/river"
)

func runInTx(ctx context.Context, dbPool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := dbPool.Begin(ctx)
	if err != nil {
		return err
	}

	err = fn(tx)
	if err == nil {
		return tx.Commit(ctx)
	}

	rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rollbackErr := tx.Rollback(rollbackCtx)
	if rollbackErr != nil {
		return fmt.Errorf("failed to rollback DB changes; got: %w; %w", err, rollbackErr)
	}

	return err
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

func (s *Store) Transact(ctx context.Context, txFunc func(adapters cisco.Adapters) error) error {
	return runInTx(ctx, s.dbPool, func(tx pgx.Tx) error {
		adapters := cisco.Adapters{
			AuditLogRepository: auditriverx.NewAuditLogJobRepository(tx, s.riverClient),

			CredentialRepository:       NewCredentialRepository(tx, s.encryptor, s.blindIndex),
			NetworkDeviceRepository:    NewNetworkDeviceRepository(tx),
			NetworkDeviceJobRepository: NewNetworkDeviceJobRepository(tx, s.riverClient),
		}

		return txFunc(adapters)
	})
}
