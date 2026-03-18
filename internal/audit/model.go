package audit

import (
	"fmt"
	"time"

	"github.com/gofrs/uuid/v5"
)

type AuditLog struct {
	id           uuid.UUID
	createdAt    time.Time
	action       Action
	userID       *uuid.UUID
	resourceID   uuid.UUID
	resourceType string
	status       Status
	metadata     map[string]string
	errorCode    *string
}

type Action string

const (
	ActionCreated                Action = "created"
	ActionUpdated                Action = "updated"
	ActionDeleted                Action = "deleted"
	ActionExecuted               Action = "executed"
	ActionAuthenticated          Action = "authenticated"
	ActionActivated              Action = "activated"
	ActionPasswordReset          Action = "password_reset"
	ActionActivationRequested    Action = "activation_requested"
	ActionPasswordResetRequested Action = "password_reset_requested"
)

type Status string

const (
	StatusSuccess Status = "success"
	StatusFailure Status = "failure"
)

func (al *AuditLog) ID() uuid.UUID {
	return al.id
}

func (al *AuditLog) CreatedAt() time.Time {
	return al.createdAt
}

func (al *AuditLog) Action() Action {
	return al.action
}

func (al *AuditLog) UserID() *uuid.UUID {
	return al.userID
}

func (al *AuditLog) ResourceID() uuid.UUID {
	return al.resourceID
}

func (al *AuditLog) ResourceType() string {
	return al.resourceType
}

func (al *AuditLog) Status() Status {
	return al.status
}

func (al *AuditLog) Metadata() map[string]string {
	return al.metadata
}

func (al *AuditLog) ErrorCode() *string {
	return al.errorCode
}

func NewAuditLog(action Action, userID *uuid.UUID, resourceID uuid.UUID, resourceType string, status Status, metadata map[string]string, errorCode *string) (*AuditLog, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID: %w", err)
	}

	return &AuditLog{
		id:           id,
		createdAt:    time.Now().UTC(),
		action:       action,
		userID:       userID,
		resourceID:   resourceID,
		resourceType: resourceType,
		status:       status,
		metadata:     metadata,
		errorCode:    errorCode,
	}, nil
}

func UnmarshalAuditLog(id uuid.UUID, createdAt time.Time, action Action, userID *uuid.UUID, resourceID uuid.UUID, resourceType string, status Status, metadata map[string]string, errorCode *string) *AuditLog {
	return &AuditLog{
		id:           id,
		createdAt:    createdAt,
		action:       action,
		userID:       userID,
		resourceID:   resourceID,
		resourceType: resourceType,
		status:       status,
		metadata:     metadata,
		errorCode:    errorCode,
	}
}
