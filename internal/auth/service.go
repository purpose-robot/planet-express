package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/purpose-robot/planet-express/internal/audit"
	"github.com/purpose-robot/planet-express/internal/errorz"
	"github.com/purpose-robot/planet-express/internal/krypto"
)

type Adapters struct {
	AuditLogRepository   audit.AuditLogRepository
	UserRepository       UserRepository
	EmailJobRepository   EmailJobRepository
	PermissionRepository PermissionRepository
}

type store interface {
	Transact(ctx context.Context, txFunc func(adapters Adapters) error) error
}

type UserRepository interface {
	InsertToken(ctx context.Context, token *Token) error
	InsertUserAndToken(ctx context.Context, user *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetForToken(ctx context.Context, scope Scope, hash Hash) (*User, error)
	DeleteAllForUser(ctx context.Context, scope Scope, userID uuid.UUID) error
	UpdateByID(ctx context.Context, id uuid.UUID, updateFn func(user *User) error) error
}

type EmailJobRepository interface {
	InsertActivationEmail(ctx context.Context, email, token string, expiry time.Duration, userID string) error
	InsertResetPasswordEmail(ctx context.Context, email, token string, expiry time.Duration, userID string) error
}

type PermissionRepository interface {
	AddForUser(ctx context.Context, id uuid.UUID, code ...string) error
	GetAllForUser(ctx context.Context, userID uuid.UUID) (Permissions, error)
}

type Service struct {
	store  store
	config ServiceConfig
	logger *slog.Logger
	// comparisonHash is used to compare passwords when no user was found to mitigate timing attacks.
	comparisonHash krypto.Argon2Hash
	// auditLogRepository is used for writing audit logs outside of transactions using the outbox pattern.
	auditLogRepository audit.AuditLogRepository
	// userRepository and permissionRepository are used in case the SQL query should not be ran in transaction.
	userRepository       UserRepository
	permissionRepository PermissionRepository
}

type ServiceConfig struct {
	DefaultPermissions   []Permission
	ActivationExpiry     time.Duration
	PasswordResetExpiry  time.Duration
	AuthenticationExpiry time.Duration
}

func NewService(store store, config ServiceConfig, logger *slog.Logger, auditLogRepository audit.AuditLogRepository, userRepository UserRepository, permissionRepository PermissionRepository) (*Service, error) {
	plain, err := krypto.GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token; got: %w", err)
	}

	comparisonHash, err := krypto.HashArgon2(plain[:])
	if err != nil {
		return nil, fmt.Errorf("failed to hash plaintext; got: %w", err)
	}

	return &Service{
		store:                store,
		config:               config,
		logger:               logger,
		comparisonHash:       comparisonHash,
		auditLogRepository:   auditLogRepository,
		userRepository:       userRepository,
		permissionRepository: permissionRepository,
	}, nil
}

func (svc *Service) Authenticate(ctx context.Context, scope Scope, hash Hash) (*User, error) {
	user, err := svc.userRepository.GetForToken(ctx, scope, hash)
	return user, mapDomainError(err)
}

func (svc *Service) FetchUserPermissions(ctx context.Context, userID uuid.UUID) (Permissions, error) {
	permissions, err := svc.permissionRepository.GetAllForUser(ctx, userID)
	return permissions, mapDomainError(err)
}

type RegisterUser struct {
	Email    string
	Password string
}

func (svc *Service) RegisterUser(ctx context.Context, cmd RegisterUser) error {
	user, err := NewUser(cmd.Email, cmd.Password)
	if err != nil {
		return mapDomainError(err)
	}

	err = user.RequestActivation(svc.config.ActivationExpiry)
	if err != nil {
		return mapDomainError(err)
	}

	err = svc.store.Transact(ctx, func(adapters Adapters) error {
		err := adapters.UserRepository.InsertUserAndToken(ctx, user)
		if err != nil {
			return err
		}

		code := make([]string, len(svc.config.DefaultPermissions))

		for index, permission := range svc.config.DefaultPermissions {
			code[index] = string(permission)
		}

		err = adapters.PermissionRepository.AddForUser(ctx, user.ID(), code...)
		if err != nil {
			return err
		}

		err = adapters.EmailJobRepository.InsertActivationEmail(
			ctx, user.Email(), user.Token().Plain(), svc.config.ActivationExpiry, user.ID().String(),
		)
		if err != nil {
			return err
		}

		return audit.LogSuccess(ctx, adapters.AuditLogRepository, audit.ActionCreated, new(user.ID()), user.ID(), "users", nil, nil)
	})

	return mapDomainError(err)
}

type ActivateUser struct {
	Token string
}

func (svc *Service) ActivateUser(ctx context.Context, cmd ActivateUser) error {
	hashedToken, err := ParseToken(cmd.Token)
	if err != nil {
		return mapDomainError(err)
	}

	err = svc.store.Transact(ctx, func(adapters Adapters) error {
		user, err := adapters.UserRepository.GetForToken(ctx, ScopeActivation, hashedToken)
		if err != nil {
			if !errors.Is(err, errorz.ErrRecordNotFound) {
				return err
			}

			return errorz.NewValidationFailed(
				"invalid or expired activation token",
				err,
				map[string]string{"token": "invalid or expired token"},
			)
		}

		err = adapters.UserRepository.UpdateByID(ctx, user.ID(), func(u *User) error {
			return u.Activate()
		})
		if err != nil {
			if errors.Is(err, ErrUserActivated) {
				return errorz.NewConflict("user is already activated", err)
			}

			return err
		}

		err = adapters.UserRepository.DeleteAllForUser(ctx, ScopeActivation, user.ID())
		if err != nil {
			return err
		}

		return audit.LogSuccess(ctx, adapters.AuditLogRepository, audit.ActionActivated, new(user.ID()), user.ID(), "users", nil, nil)
	})

	return mapDomainError(err)
}

type AuthenticateUser struct {
	Email    string
	Password string
}

type AuthenticationResponse struct {
	Token  string
	Expiry time.Time
}

func (svc *Service) AuthenticateUser(ctx context.Context, cmd AuthenticateUser) (*AuthenticationResponse, error) {
	email, err := ParseEmail(cmd.Email)
	if err != nil {
		return nil, mapDomainError(err)
	}

	if cmd.Password == "" {
		err := errorz.ValidationFailed{Field: "password", Message: "cannot be empty"}
		return nil, mapDomainError(err)
	}

	var userID *uuid.UUID
	response := new(AuthenticationResponse)

	err = svc.store.Transact(ctx, func(adapters Adapters) error {
		user, err := adapters.UserRepository.GetByEmail(ctx, email)
		if err != nil {
			if !errors.Is(err, errorz.ErrRecordNotFound) {
				return err
			}

			_ = svc.comparisonHash.MatchBytes([]byte(cmd.Password))
			return NewInvalidCredentials("incorrect email address or password", ErrInvalidCredentials)
		}

		userID = new(user.ID())

		if !user.MatchPassword(cmd.Password) {
			return NewInvalidCredentials("incorrect email address or password", ErrInvalidCredentials)
		}

		err = user.RequestAuthentication(svc.config.AuthenticationExpiry)
		if err != nil {
			if !errors.Is(err, ErrUserNotActivated) {
				return err
			}

			return NewInvalidCredentials("incorrect email address or password", ErrInvalidCredentials)
		}

		err = adapters.UserRepository.InsertToken(ctx, user.Token())
		if err != nil {
			return err
		}

		response.Token = user.Token().Plain()
		response.Expiry = user.Token().Expiry()

		return audit.LogSuccess(ctx, adapters.AuditLogRepository, audit.ActionAuthenticated, userID, *userID, "users", nil, nil)
	})
	if err != nil {
		if safeErr, ok := errors.AsType[*errorz.SafeError](err); ok && safeErr.Code == errorz.CodeInvalidCredentials {
			resourceID := uuid.Nil
			if userID != nil {
				resourceID = *userID
			}

			auditErr := audit.LogFailure(ctx, svc.auditLogRepository, audit.ActionAuthenticated, userID, resourceID, "users", nil, &safeErr.Code)
			if auditErr != nil {
				svc.logger.ErrorContext(ctx, "failed to write audit log", slog.Any("error", auditErr))
			}
		}

		return nil, mapDomainError(err)
	}

	return response, nil
}

type SendActivationToken struct {
	Email string
}

func (svc *Service) SendActivationToken(ctx context.Context, cmd SendActivationToken) error {
	email, err := ParseEmail(cmd.Email)
	if err != nil {
		return mapDomainError(err)
	}

	err = svc.store.Transact(ctx, func(adapters Adapters) error {
		user, err := adapters.UserRepository.GetByEmail(ctx, email)
		if err != nil {
			if !errors.Is(err, errorz.ErrRecordNotFound) {
				return err
			}

			return errorz.NewValidationFailed(
				"invalid email address",
				err,
				map[string]string{"email": "no matching email address found"},
			)
		}

		err = user.RequestActivation(svc.config.ActivationExpiry)
		if err != nil {
			if !errors.Is(err, ErrUserActivated) {
				return err
			}

			return errorz.NewValidationFailed(
				"account is already activated",
				err,
				map[string]string{"email": "this account has already been activated"},
			)
		}

		err = adapters.UserRepository.InsertToken(ctx, user.Token())
		if err != nil {
			return err
		}

		err = adapters.EmailJobRepository.InsertActivationEmail(
			ctx, user.Email(), user.Token().Plain(), svc.config.ActivationExpiry, user.ID().String(),
		)
		if err != nil {
			return err
		}

		return audit.LogSuccess(ctx, adapters.AuditLogRepository, audit.ActionActivationRequested, new(user.ID()), user.ID(), "users", nil, nil)
	})

	return mapDomainError(err)
}

type ResetPassword struct {
	Token    string
	Password string
}

func (svc *Service) ResetPassword(ctx context.Context, cmd ResetPassword) error {
	hashedToken, err := ParseToken(cmd.Token)
	if err != nil {
		return mapDomainError(err)
	}

	err = svc.store.Transact(ctx, func(adapters Adapters) error {
		user, err := adapters.UserRepository.GetForToken(ctx, ScopePasswordReset, hashedToken)
		if err != nil {
			if !errors.Is(err, errorz.ErrRecordNotFound) {
				return err
			}

			return errorz.NewValidationFailed(
				"invalid or expired password reset token",
				err,
				map[string]string{"token": "invalid or expired token"},
			)
		}

		err = adapters.UserRepository.UpdateByID(ctx, user.ID(), func(u *User) error {
			return u.ResetPassword(cmd.Password)
		})
		if err != nil {
			return err
		}

		err = adapters.UserRepository.DeleteAllForUser(ctx, ScopePasswordReset, user.ID())
		if err != nil {
			return err
		}

		return audit.LogSuccess(ctx, adapters.AuditLogRepository, audit.ActionPasswordReset, new(user.ID()), user.ID(), "users", nil, nil)
	})

	return mapDomainError(err)
}

type RequestPasswordReset struct {
	Email string
}

func (svc *Service) RequestPasswordReset(ctx context.Context, cmd RequestPasswordReset) error {
	email, err := ParseEmail(cmd.Email)
	if err != nil {
		return mapDomainError(err)
	}

	err = svc.store.Transact(ctx, func(adapters Adapters) error {
		user, err := adapters.UserRepository.GetByEmail(ctx, email)
		if err != nil {
			if !errors.Is(err, errorz.ErrRecordNotFound) {
				return err
			}

			return errorz.NewValidationFailed(
				"invalid email address",
				err,
				map[string]string{"email": "no matching email address found"},
			)
		}

		err = user.RequestPasswordReset(svc.config.PasswordResetExpiry)
		if err != nil {
			if !errors.Is(err, ErrUserNotActivated) {
				return err
			}

			return errorz.NewValidationFailed(
				"account is not activated",
				err,
				map[string]string{"email": "this account has not been activated"},
			)
		}

		err = adapters.UserRepository.InsertToken(ctx, user.Token())
		if err != nil {
			return err
		}

		err = adapters.EmailJobRepository.InsertResetPasswordEmail(
			ctx, user.Email(), user.Token().Plain(), svc.config.PasswordResetExpiry, user.ID().String(),
		)
		if err != nil {
			return err
		}

		return audit.LogSuccess(ctx, adapters.AuditLogRepository, audit.ActionPasswordResetRequested, new(user.ID()), user.ID(), "users", nil, nil)
	})

	return mapDomainError(err)
}
