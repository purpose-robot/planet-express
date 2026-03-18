package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/purpose-robot/planet-express/internal/auth"
	"github.com/riverqueue/river"
)

type EmailJobRepository struct {
	tx          pgx.Tx
	riverClient *river.Client[pgx.Tx]
}

func NewEmailJobRepository(tx pgx.Tx, riverClient *river.Client[pgx.Tx]) *EmailJobRepository {
	return &EmailJobRepository{
		tx:          tx,
		riverClient: riverClient,
	}
}

func (r *EmailJobRepository) InsertActivationEmail(ctx context.Context, email, token string, expiry time.Duration, userID string) error {
	args := auth.SendActivationEmailArgs{
		Email:  email,
		Token:  token,
		Expiry: expiry,
		UserID: userID,
	}

	_, err := r.riverClient.InsertTx(ctx, r.tx, args, nil)
	return err
}

func (r *EmailJobRepository) InsertResetPasswordEmail(ctx context.Context, email, token string, expiry time.Duration, userID string) error {
	args := auth.SendPasswordResetEmailArgs{
		Email:  email,
		Token:  token,
		Expiry: expiry,
		UserID: userID,
	}

	_, err := r.riverClient.InsertTx(ctx, r.tx, args, nil)
	return err
}
