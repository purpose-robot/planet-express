package errorz

import (
	"errors"
	"fmt"
)

var (
	ErrConflict         = errors.New("conflict")
	ErrInternal         = errors.New("internal")
	ErrNotFound         = errors.New("not found")
	ErrRecordNotFound   = fmt.Errorf("record not found: %w", ErrNotFound)
	ErrValidationFailed = errors.New("failed validation")
)

type ValidationFailed struct {
	Field   string
	Message string
}

func (e ValidationFailed) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

const (
	CodeConflict           = "CONFLICT"
	CodeInternal           = "INTERNAL"
	CodeNotFound           = "NOT_FOUND"
	CodeValidationFailed   = "VALIDATION_FAILED"
	CodeInvalidCredentials = "INVALID_CREDENTIALS"
)

type SafeError struct {
	Code     string
	UserMsg  string
	Internal error
	Metadata map[string]string
}

func (e *SafeError) Error() string {
	return e.UserMsg
}

func (e *SafeError) LogString() string {
	return fmt.Sprintf("Code: %s | Msg: %s | Cause: %v | Meta: %v", e.Code, e.UserMsg, e.Internal, e.Metadata)
}

func NewConflict(userMsg string, internal error) *SafeError {
	return &SafeError{
		Code:     CodeConflict,
		UserMsg:  userMsg,
		Internal: fmt.Errorf("%w: %w", ErrConflict, internal),
	}
}

func NewNotFound(userMsg string, internal error) *SafeError {
	return &SafeError{
		Code:     CodeNotFound,
		UserMsg:  userMsg,
		Internal: fmt.Errorf("%w: %w", ErrNotFound, internal),
	}
}

func NewValidationFailed(userMsg string, internal error, metadata map[string]string) *SafeError {
	return &SafeError{
		Code:     CodeValidationFailed,
		UserMsg:  userMsg,
		Internal: internal,
		Metadata: metadata,
	}
}

func NewInternal(internal error) *SafeError {
	return &SafeError{
		Code:     CodeInternal,
		UserMsg:  "the server encountered an unexpected condition and could not process your request",
		Internal: fmt.Errorf("%w: %w", ErrInternal, internal),
	}
}
