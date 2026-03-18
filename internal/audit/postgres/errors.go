package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
		case "plnx_audit_logs_status_check":
			return errorz.NewInternal(fmt.Errorf("invalid audit log status: %s", postgresErr.Message))

		case "plnx_audit_logs_action_check":
			return errorz.NewInternal(fmt.Errorf("invalid audit log action: %s", postgresErr.Message))
		}
	}

	return fmt.Errorf("unexpected database error: %s", err)
}
