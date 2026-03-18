package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gofrs/uuid/v5"
	"github.com/purpose-robot/planet-express/internal/auth"
	"github.com/purpose-robot/planet-express/internal/errorz"
	"github.com/purpose-robot/planet-express/internal/httpz"
)

type service interface {
	FetchUserPermissions(ctx context.Context, userID uuid.UUID) (auth.Permissions, error)
	Authenticate(ctx context.Context, scope auth.Scope, hash auth.Hash) (*auth.User, error)
}

type Middleware struct {
	logger  *slog.Logger
	service service
}

func NewMiddleware(logger *slog.Logger, service service) *Middleware {
	return &Middleware{
		logger:  logger,
		service: service,
	}
}

func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Authorization")

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			next.ServeHTTP(w, r)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			httpz.InvalidAuthenticationTokenResponse(w)
			return
		}

		hashedToken, err := auth.ParseToken(parts[1])
		if err != nil {
			httpz.InvalidAuthenticationTokenResponse(w)
			return
		}

		user, err := m.service.Authenticate(r.Context(), auth.ScopeAuthentication, hashedToken)
		if err != nil {
			if safeErr, ok := errors.AsType[*errorz.SafeError](err); !ok {
				m.logger.ErrorContext(r.Context(), "failed to authenticate user", slog.Any("error", err))
			} else {
				if safeErr.Code == errorz.CodeNotFound {
					httpz.InvalidAuthenticationTokenResponse(w)
					return
				}

				m.logger.ErrorContext(r.Context(), "failed to authenticate user", slog.String("error", safeErr.LogString()))
			}

			httpz.InternalServerErrorResponse(w)
			return
		}

		next.ServeHTTP(w, contextSetAuthenticatedUser(r, user))
	})
}

func (m *Middleware) RequirePermission(code auth.Permission, next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticatedUser, found := ContextGetAuthenticatedUser(r)
		if !found {
			httpz.AuthenticationRequiredResponse(w)
			return
		}

		if !authenticatedUser.Activated() {
			httpz.InactiveAccountResponse(w)
			return
		}

		permissions, err := m.service.FetchUserPermissions(r.Context(), authenticatedUser.ID())
		if err != nil {
			if safeErr, ok := errors.AsType[*errorz.SafeError](err); !ok {
				m.logger.ErrorContext(r.Context(), "failed to get permissions for user", slog.Any("error", err))
			} else {
				m.logger.ErrorContext(r.Context(), "failed to get permissions for user", slog.String("error", safeErr.LogString()))
			}

			httpz.InternalServerErrorResponse(w)
			return
		}

		if !permissions.Include(code) {
			httpz.MissingPermissionResponse(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}
