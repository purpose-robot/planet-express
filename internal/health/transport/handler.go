package transport

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/purpose-robot/planet-express/internal/errorz"
	"github.com/purpose-robot/planet-express/internal/health"
	"github.com/purpose-robot/planet-express/internal/httpz"
)

type service interface {
	CheckHealth() (health.CheckHealthResponse, error)
}

type Handler struct {
	logger  *slog.Logger
	service service
}

func NewHandler(logger *slog.Logger, service service) *Handler {
	return &Handler{
		logger:  logger,
		service: service,
	}
}

func (h *Handler) CheckHealth(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.CheckHealth()
	if err != nil {
		h.mapHTTPError(w, r, err)
		return
	}

	type output struct {
		Status  string `json:"status"`
		Uptime  string `json:"uptime"`
		Version string `json:"version"`
	}

	err = httpz.WriteJSON(w, http.StatusOK, httpz.Envelope{"health": output{
		Status:  result.Status,
		Uptime:  result.Uptime,
		Version: result.Version,
	}}, nil)
	if err != nil {
		h.mapHTTPError(w, r, err)
	}
}

func (h *Handler) mapHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	safeErr, ok := errors.AsType[*errorz.SafeError](err)
	if !ok {
		h.logger.ErrorContext(r.Context(), "non-safe error reached HTTP handler", slog.Any("error", err))
		httpz.InternalServerErrorResponse(w)
		return
	}

	switch safeErr.Code {
	default:
		h.logger.ErrorContext(r.Context(), "unhandled service error", slog.Any("error", safeErr.Internal))
		httpz.InternalServerErrorResponse(w)
	}
}
