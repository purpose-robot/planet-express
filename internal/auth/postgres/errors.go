package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/purpose-robot/planet-express/internal/auth"
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
		case "plnx_users_email_blind_index_key":
			return fmt.Errorf("%w: %s", auth.ErrDuplicateEmailAddress, postgresErr.Message)
		}
	}

	return fmt.Errorf("unexpected database error: %w", err)
}
