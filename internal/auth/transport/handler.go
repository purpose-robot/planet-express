package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/purpose-robot/planet-express/internal/auth"
	"github.com/purpose-robot/planet-express/internal/errorz"
	"github.com/purpose-robot/planet-express/internal/httpz"
)

type service interface {
	RegisterUser(ctx context.Context, cmd auth.RegisterUser) error
	ActivateUser(ctx context.Context, cmd auth.ActivateUser) error
	AuthenticateUser(ctx context.Context, cmd auth.AuthenticateUser) (*auth.AuthenticationResponse, error)
	SendActivationToken(ctx context.Context, cmd auth.SendActivationToken) error
	ResetPassword(ctx context.Context, cmd auth.ResetPassword) error
	RequestPasswordReset(ctx context.Context, cmd auth.RequestPasswordReset) error
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

func (h *Handler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	type input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	var in input

	err := httpz.ReadJSON(w, r, &in)
	if err != nil {
		httpz.BadRequestResponse(w, err.Error())
		return
	}

	err = h.service.RegisterUser(r.Context(), auth.RegisterUser{
		Email:    in.Email,
		Password: in.Password,
	})
	if err != nil {
		h.mapHTTPError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) ActivateUser(w http.ResponseWriter, r *http.Request) {
	type input struct {
		Token string `json:"token"`
	}

	var in input

	err := httpz.ReadJSON(w, r, &in)
	if err != nil {
		httpz.BadRequestResponse(w, err.Error())
		return
	}

	err = h.service.ActivateUser(r.Context(), auth.ActivateUser{
		Token: in.Token,
	})
	if err != nil {
		h.mapHTTPError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AuthenticateUser(w http.ResponseWriter, r *http.Request) {
	type input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	var in input

	err := httpz.ReadJSON(w, r, &in)
	if err != nil {
		httpz.BadRequestResponse(w, err.Error())
		return
	}

	result, err := h.service.AuthenticateUser(r.Context(), auth.AuthenticateUser{
		Email:    in.Email,
		Password: in.Password,
	})
	if err != nil {
		h.mapHTTPError(w, r, err)
		return
	}

	type output struct {
		Token  string    `json:"token"`
		Expiry time.Time `json:"expiry"`
	}

	err = httpz.WriteJSON(w, http.StatusCreated, httpz.Envelope{"auth": output{
		Token:  result.Token,
		Expiry: result.Expiry,
	}}, nil)
	if err != nil {
		h.mapHTTPError(w, r, err)
	}
}

func (h *Handler) SendActivationToken(w http.ResponseWriter, r *http.Request) {
	type input struct {
		Email string `json:"email"`
	}

	var in input

	err := httpz.ReadJSON(w, r, &in)
	if err != nil {
		httpz.BadRequestResponse(w, err.Error())
		return
	}

	err = h.service.SendActivationToken(r.Context(), auth.SendActivationToken{
		Email: in.Email,
	})

	if err != nil {
		h.mapHTTPError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	type input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}

	var in input

	err := httpz.ReadJSON(w, r, &in)
	if err != nil {
		httpz.BadRequestResponse(w, err.Error())
		return
	}

	err = h.service.ResetPassword(r.Context(), auth.ResetPassword{
		Token:    in.Token,
		Password: in.Password,
	})
	if err != nil {
		h.mapHTTPError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	type input struct {
		Email string `json:"email"`
	}

	var in input

	err := httpz.ReadJSON(w, r, &in)
	if err != nil {
		httpz.BadRequestResponse(w, err.Error())
		return
	}

	err = h.service.RequestPasswordReset(r.Context(), auth.RequestPasswordReset{
		Email: in.Email,
	})
	if err != nil {
		h.mapHTTPError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) mapHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	safeErr, ok := errors.AsType[*errorz.SafeError](err)
	if !ok {
		h.logger.ErrorContext(r.Context(), "non-safe error reached HTTP handler", slog.Any("error", err))
		httpz.InternalServerErrorResponse(w)
		return
	}

	switch safeErr.Code {
	case errorz.CodeConflict:
		httpz.ConflictResponse(w, safeErr.UserMsg)

	case errorz.CodeNotFound:
		httpz.NotFoundResponse(w, safeErr.UserMsg)

	case errorz.CodeValidationFailed:
		httpz.FailedValidationResponse(w, safeErr.UserMsg, safeErr.Metadata)

	case errorz.CodeInvalidCredentials:
		httpz.InvalidCredentialsResponse(w, safeErr.UserMsg)

	default:
		h.logger.ErrorContext(r.Context(), "unhandled service error", slog.Any("error", safeErr.LogString()))
		httpz.InternalServerErrorResponse(w)
	}
}
