package audit

import (
	"context"

	"github.com/gofrs/uuid/v5"
)

type AuditLogRepository interface {
	Insert(ctx context.Context, auditLog *AuditLog) error
}

func LogSuccess(ctx context.Context, repo AuditLogRepository, action Action, userID *uuid.UUID, resourceID uuid.UUID, resourceType string, metadata map[string]string, errorCode *string) error {
	return log(ctx, repo, action, userID, resourceID, resourceType, StatusSuccess, metadata, errorCode)
}

func LogFailure(ctx context.Context, repo AuditLogRepository, action Action, userID *uuid.UUID, resourceID uuid.UUID, resourceType string, metadata map[string]string, errorCode *string) error {
	return log(ctx, repo, action, userID, resourceID, resourceType, StatusFailure, metadata, errorCode)
}

func log(ctx context.Context, repo AuditLogRepository, action Action, userID *uuid.UUID, resourceID uuid.UUID, resourceType string, status Status, metadata map[string]string, errorCode *string) error {
	auditLog, err := NewAuditLog(action, userID, resourceID, resourceType, status, metadata, errorCode)
	if err != nil {
		return err
	}

	return repo.Insert(ctx, auditLog)
}
