package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/purpose-robot/planet-express/internal/cisco"
	"github.com/purpose-robot/planet-express/internal/errorz"
)

func mapRepositoryError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return errorz.ErrRecordNotFound
	}

	if postgresErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		switch postgresErr.ConstraintName {
		case "plnx_credentials_auth_method_local_unique":
			return fmt.Errorf("%w: %s", cisco.ErrDuplicateAuthMethod, postgresErr.Message)
		}
	}

	return fmt.Errorf("unexpected database error: %w", err)
}
