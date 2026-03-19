package postgres

import (
	"context"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/purpose-robot/planet-express/internal/cisco"
	"github.com/riverqueue/river"
)

type NetworkDeviceJobRepository struct {
	tx          pgx.Tx
	riverClient *river.Client[pgx.Tx]
}

func NewNetworkDeviceJobRepository(tx pgx.Tx, riverClient *river.Client[pgx.Tx]) *NetworkDeviceJobRepository {
	return &NetworkDeviceJobRepository{
		tx:          tx,
		riverClient: riverClient,
	}
}

func (r *NetworkDeviceJobRepository) CreateNetworkDeviceSyncJob(ctx context.Context, credentialID, networkDeviceID uuid.UUID) error {
	args := cisco.SyncNetworkDeviceArgs{
		CredentialID:    credentialID,
		NetworkDeviceID: networkDeviceID,
	}

	_, err := r.riverClient.InsertTx(ctx, r.tx, args, nil)
	return err
}
