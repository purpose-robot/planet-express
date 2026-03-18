package riverx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gofrs/uuid/v5"
	"github.com/purpose-robot/planet-express/internal/audit"
	"github.com/purpose-robot/planet-express/internal/errorz"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

type ErrorHandler struct {
	repo   audit.AuditLogRepository
	logger *slog.Logger
}

func NewErrorHandler(logger *slog.Logger, repo audit.AuditLogRepository) *ErrorHandler {
	return &ErrorHandler{
		repo:   repo,
		logger: logger,
	}
}

func (h *ErrorHandler) insertAuditLog(ctx context.Context, jobKind string, metadata map[string]string, errorCode *string) {
	auditLog, err := audit.NewAuditLog(audit.ActionExecuted, nil, uuid.Nil, jobKind, audit.StatusFailure, metadata, errorCode)
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to create audit log", slog.String("job_kind", jobKind), slog.Any("error", err))
		return
	}

	insertErr := h.repo.Insert(ctx, auditLog)
	if insertErr != nil {
		h.logger.ErrorContext(ctx, "failed to insert audit log", slog.String("job_kind", jobKind), slog.Any("error", insertErr))
	}
}

func (h *ErrorHandler) HandleError(ctx context.Context, job *rivertype.JobRow, err error) *river.ErrorHandlerResult {
	safeErr, ok := errors.AsType[*errorz.SafeError](err)

	if job.Attempt >= job.MaxAttempts {
		internalErr := err.Error()
		if ok && safeErr.Internal != nil {
			internalErr = safeErr.Internal.Error()
		}

		h.insertAuditLog(ctx, job.Kind, map[string]string{"job_id": strconv.FormatInt(job.ID, 10), "error": internalErr}, new("JOB_FAILED"))
	}

	return nil
}

func (h *ErrorHandler) HandlePanic(ctx context.Context, job *rivertype.JobRow, panicVal any, trace string) *river.ErrorHandlerResult {
	h.logger.ErrorContext(ctx, "job panicked", slog.String("job_kind", job.Kind), slog.Any("panic", panicVal), slog.String("trace", trace))

	h.insertAuditLog(ctx, job.Kind, map[string]string{"job_id": strconv.FormatInt(job.ID, 10), "panic": fmt.Sprintf("%v", panicVal)}, new("JOB_PANIC"))

	return &river.ErrorHandlerResult{
		SetCancelled: true,
	}
}
