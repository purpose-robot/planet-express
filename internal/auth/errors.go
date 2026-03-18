package auth

import (
	"errors"

	"github.com/purpose-robot/planet-express/internal/errorz"
)

var (
	ErrUserActivated         = errors.New("user activated")
	ErrUserNotActivated      = errors.New("user not activated")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrDuplicateEmailAddress = errors.New("email address already exists")
)

func NewInvalidCredentials(userMsg string, internal error) *errorz.SafeError {
	return &errorz.SafeError{
		Code:     errorz.CodeInvalidCredentials,
		UserMsg:  userMsg,
		Internal: internal,
	}
}

func mapDomainError(err error) error {
	if err == nil {
		return nil
	}

	if safeErr, ok := errors.AsType[*errorz.SafeError](err); ok {
		return safeErr
	}

	if validationErr, ok := errors.AsType[errorz.ValidationFailed](err); ok {
		return errorz.NewValidationFailed(
			"one or more fields could not be validated",
			nil,
			map[string]string{validationErr.Field: validationErr.Message},
		)
	}

	if errors.Is(err, errorz.ErrNotFound) {
		return errorz.NewNotFound("the requested resource was not found", err)
	}

	if errors.Is(err, ErrDuplicateEmailAddress) {
		return errorz.NewConflict("a user with this email address already exists", err)
	}

	return errorz.NewInternal(err)
}
