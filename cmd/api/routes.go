package main

import (
	"net/http"

	authmiddleware "github.com/purpose-robot/planet-express/internal/auth/middleware"
	authtransport "github.com/purpose-robot/planet-express/internal/auth/transport"
	"github.com/purpose-robot/planet-express/internal/bessie"
	bessietransport "github.com/purpose-robot/planet-express/internal/bessie/transport"
	"github.com/purpose-robot/planet-express/internal/health"
	healthtransport "github.com/purpose-robot/planet-express/internal/health/transport"
	"github.com/purpose-robot/planet-express/internal/httpz"
)

func (app *application) routes() http.Handler {
	mux := http.NewServeMux()

	authTransport := authtransport.NewHandler(app.logger, app.authService)
	bessieTransport := bessietransport.NewHandler(app.logger, app.bessieService)
	healthTransport := healthtransport.NewHandler(app.logger, app.healthService)

	httpMiddleware := httpz.NewMiddleware(app.logger, httpz.LimiterConfig{
		RPS:     app.config.HTTP.Limiter.RPS,
		Burst:   app.config.HTTP.Limiter.Burst,
		Enabled: app.config.HTTP.Limiter.Enabled,
	})

	authMiddleware := authmiddleware.NewMiddleware(app.logger, app.authService)

	mux.HandleFunc("POST /api/v1/users", authTransport.RegisterUser)
	mux.HandleFunc("PUT /api/v1/users/activate", authTransport.ActivateUser)
	mux.HandleFunc("PUT /api/v1/users/password", authTransport.ResetPassword)

	mux.HandleFunc("POST /api/v1/tokens/activation", authTransport.SendActivationToken)
	mux.HandleFunc("POST /api/v1/tokens/authentication", authTransport.AuthenticateUser)
	mux.HandleFunc("POST /api/v1/tokens/password-reset", authTransport.RequestPasswordReset)

	mux.HandleFunc(
		"GET /api/v1/health",
		authMiddleware.RequirePermission(health.PermissionRead, healthTransport.CheckHealth),
	)

	// PUT  /api/v1/network-devices/sync
	// POST /api/v1/discovery

	mux.HandleFunc(
		"POST /api/v1/credentials",
		authMiddleware.RequirePermission(bessie.PermissionCredentialsWrite, bessieTransport.AddCredential),
	)

	mux.HandleFunc(
		"POST /api/v1/network-devices",
		authMiddleware.RequirePermission(bessie.PermissionNetworkDevicesWrite, bessieTransport.AddNetworkDevice),
	)

	return httpMiddleware.LogRequests(httpMiddleware.RecoverPanic(httpMiddleware.RateLimit(authMiddleware.Authenticate(mux))))
}
