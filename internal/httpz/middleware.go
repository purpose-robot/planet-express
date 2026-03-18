package httpz

import (
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type Middleware struct {
	logger  *slog.Logger
	limiter *rate.Limiter
}

type LimiterConfig struct {
	RPS     int
	Burst   int
	Enabled bool
}

func NewMiddleware(logger *slog.Logger, limiterConfig LimiterConfig) *Middleware {
	var limiter *rate.Limiter

	if limiterConfig.Enabled {
		limiter = rate.NewLimiter(rate.Limit(limiterConfig.RPS), limiterConfig.Burst)
	}

	return &Middleware{
		logger:  logger,
		limiter: limiter,
	}
}

func (m *Middleware) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.limiter == nil {
			next.ServeHTTP(w, r)
			return
		}

		if !m.limiter.Allow() {
			RateLimitExceededResponse(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) LogRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now().UTC()

		rr := newResponseRecorder(w)

		next.ServeHTTP(rr, r)

		m.logger.InfoContext(
			r.Context(),
			"http request",
			slog.String("method", r.Method),
			slog.String("url_path", r.URL.Path),
			slog.String("url_query", r.URL.RawQuery),
			slog.Int("status", rr.status),
			slog.Int("bytes_written", rr.bytes),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

func (m *Middleware) RecoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			pv := recover()
			if pv != nil {
				m.logger.ErrorContext(r.Context(), "panic recovered", slog.Any("error", pv))

				w.Header().Set("Connection", "close")
				InternalServerErrorResponse(w)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
