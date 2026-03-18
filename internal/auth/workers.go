package auth

import (
	"context"
	"time"

	"github.com/riverqueue/river"
)

type Emailer interface {
	Send(ctx context.Context, name, recipient string, data any) error
}

type SendActivationEmailArgs struct {
	Email  string        `json:"email"`
	Token  string        `json:"token"`
	Expiry time.Duration `json:"expiry"`
	UserID string        `json:"user_id"`
}

func (SendActivationEmailArgs) Kind() string {
	return "send_activation_email"
}

type activationEmail struct {
	Token  string
	Expiry time.Duration
	UserID string
}

type SendActivationEmailWorker struct {
	emailer Emailer
	river.WorkerDefaults[SendActivationEmailArgs]
}

func NewSendActivationEmailWorker(emailer Emailer) *SendActivationEmailWorker {
	return &SendActivationEmailWorker{
		emailer: emailer,
	}
}

func (w *SendActivationEmailWorker) Work(ctx context.Context, job *river.Job[SendActivationEmailArgs]) error {
	return w.emailer.Send(ctx, "activate-user", job.Args.Email, activationEmail{
		Token:  job.Args.Token,
		Expiry: job.Args.Expiry,
		UserID: job.Args.UserID,
	})
}

type SendPasswordResetEmailArgs struct {
	Email  string        `json:"email"`
	Token  string        `json:"token"`
	Expiry time.Duration `json:"expiry"`
	UserID string        `json:"user_id"`
}

func (SendPasswordResetEmailArgs) Kind() string {
	return "send_password_reset_email"
}

type passwordResetEmail struct {
	Token  string
	Expiry time.Duration
	UserID string
}

type SendPasswordResetEmailWorker struct {
	river.WorkerDefaults[SendPasswordResetEmailArgs]
	emailer Emailer
}

func NewSendPasswordResetEmailWorker(emailer Emailer) *SendPasswordResetEmailWorker {
	return &SendPasswordResetEmailWorker{
		emailer: emailer,
	}
}

func (w *SendPasswordResetEmailWorker) Work(ctx context.Context, job *river.Job[SendPasswordResetEmailArgs]) error {
	return w.emailer.Send(ctx, "reset-user-password", job.Args.Email, passwordResetEmail{
		Token:  job.Args.Token,
		Expiry: job.Args.Expiry,
		UserID: job.Args.UserID,
	})
}
