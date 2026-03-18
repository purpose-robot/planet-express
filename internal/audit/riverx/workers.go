package riverx

import (
	"context"

	"github.com/gofrs/uuid/v5"
	"github.com/purpose-robot/planet-express/internal/audit"
	"github.com/riverqueue/river"
)

type auditLogArgs struct {
	Action       string            `json:"action"`
	UserID       *uuid.UUID        `json:"user_id"`
	ResourceID   uuid.UUID         `json:"resource_id"`
	ResourceType string            `json:"resource_type"`
	Status       string            `json:"status"`
	Metadata     map[string]string `json:"metadata"`
	ErrorCode    *string           `json:"error_code"`
}

func (auditLogArgs) Kind() string {
	return "audit_log"
}

type AuditLogWorker struct {
	river.WorkerDefaults[auditLogArgs]
	repository audit.AuditLogRepository
}

func NewAuditLogWorker(repository audit.AuditLogRepository) *AuditLogWorker {
	return &AuditLogWorker{
		repository: repository,
	}
}

func (w *AuditLogWorker) Work(ctx context.Context, job *river.Job[auditLogArgs]) error {
	auditLog, err := audit.NewAuditLog(audit.Action(job.Args.Action), job.Args.UserID, job.Args.ResourceID, job.Args.ResourceType, audit.Status(job.Args.Status), job.Args.Metadata, job.Args.ErrorCode)
	if err != nil {
		return err
	}

	return w.repository.Insert(ctx, auditLog)
}
