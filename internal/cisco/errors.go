package cisco

import (
	"errors"

	"github.com/purpose-robot/planet-express/internal/errorz"
)

var (
	ErrDuplicateAuthMethod = errors.New("credentials already exists")
	// The following errors are used to provide better error handling with SSH.
	ErrDiscovererInputFailed      = errors.New("discoverer: input failed")
	ErrDiscovererInvalidOptions   = errors.New("discoverer: invalid options")
	ErrDiscovererConnectionLost   = errors.New("discoverer: connection lost")
	ErrDiscovererConnectionFailed = errors.New("discoverer: failed to connect")
	ErrDiscovererInvalidTemplate  = errors.New("discoverer: invalid template")
	ErrDiscovererUnexpectedOutput = errors.New("discoverer: unexpected output")
)

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

	if errors.Is(err, ErrDuplicateAuthMethod) {
		return errorz.NewConflict("credentials for local auth method already exist", err)
	}

	return errorz.NewInternal(err)
}
