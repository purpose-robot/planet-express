package middleware

import (
	"context"
	"net/http"

	"github.com/purpose-robot/planet-express/internal/auth"
)

type contextKey string

const authenticatedUserContextKey = contextKey("authenticatedUser")

func ContextGetAuthenticatedUser(r *http.Request) (*auth.User, bool) {
	user, ok := r.Context().Value(authenticatedUserContextKey).(*auth.User)
	return user, ok
}

func contextSetAuthenticatedUser(r *http.Request, user *auth.User) *http.Request {
	ctx := context.WithValue(r.Context(), authenticatedUserContextKey, user)
	return r.WithContext(ctx)
}
