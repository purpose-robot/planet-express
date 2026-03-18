package riverx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/purpose-robot/planet-express/internal/audit"
	"github.com/riverqueue/river"
)

type AuditLogJobRepository struct {
	tx          pgx.Tx
	riverClient *river.Client[pgx.Tx]
}

func NewAuditLogJobRepository(tx pgx.Tx, riverClient *river.Client[pgx.Tx]) *AuditLogJobRepository {
	return &AuditLogJobRepository{
		tx:          tx,
		riverClient: riverClient,
	}
}

func (r *AuditLogJobRepository) Insert(ctx context.Context, auditLog *audit.AuditLog) error {
	args := auditLogArgs{
		Action:       string(auditLog.Action()),
		UserID:       auditLog.UserID(),
		ResourceID:   auditLog.ResourceID(),
		ResourceType: auditLog.ResourceType(),
		Status:       string(auditLog.Status()),
		Metadata:     auditLog.Metadata(),
		ErrorCode:    auditLog.ErrorCode(),
	}

	_, err := r.riverClient.InsertTx(ctx, r.tx, args, nil)
	return err
}
