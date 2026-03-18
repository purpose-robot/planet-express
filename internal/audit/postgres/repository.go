package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/purpose-robot/planet-express/internal/audit"
)

type dbPool interface {
	Exec(ctx context.Context, stmt string, namedArgs ...any) (pgconn.CommandTag, error)
}

type AuditLogRepository struct {
	dbPool dbPool
}

func NewAuditLogRepository(dbPool dbPool) *AuditLogRepository {
	return &AuditLogRepository{
		dbPool: dbPool,
	}
}

func (r *AuditLogRepository) Insert(ctx context.Context, auditLog *audit.AuditLog) error {
	stmt := `
		INSERT INTO plnx_audit_logs (
			id, created_at, action, user_id, resource_id, resource_type, status, metadata, error_code
		) VALUES (
			@id, @created_at, @action, @user_id, @resource_id, @resource_type, @status, @metadata, @error_code
		)`

	namedArgs := pgx.NamedArgs{
		"id":            auditLog.ID(),
		"created_at":    auditLog.CreatedAt(),
		"action":        auditLog.Action(),
		"user_id":       auditLog.UserID(),
		"resource_id":   auditLog.ResourceID(),
		"resource_type": auditLog.ResourceType(),
		"status":        auditLog.Status(),
		"metadata":      auditLog.Metadata(),
		"error_code":    auditLog.ErrorCode(),
	}

	_, err := r.dbPool.Exec(ctx, stmt, namedArgs)
	return mapRepositoryError(err)
}
