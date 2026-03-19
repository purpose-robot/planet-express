package transport

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"

	"github.com/gofrs/uuid/v5"
	authmiddleware "github.com/purpose-robot/planet-express/internal/auth/middleware"
	"github.com/purpose-robot/planet-express/internal/cisco"
	"github.com/purpose-robot/planet-express/internal/errorz"
	"github.com/purpose-robot/planet-express/internal/httpz"
)

type service interface {
	AddCredential(ctx context.Context, cmd cisco.AddCredential) (cisco.AddCredentialResult, error)
	AddNetworkDevice(ctx context.Context, cmd cisco.AddNetworkDevice) (cisco.AddNetworkDeviceResult, error)
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

func (h *Handler) AddCredential(w http.ResponseWriter, r *http.Request) {
	type input struct {
		Username    string  `json:"username"`
		Password    string  `json:"password"`
		AuthMethod  string  `json:"auth_method"`
		Description *string `json:"description"`
	}

	var in input

	err := httpz.ReadJSON(w, r, &in)
	if err != nil {
		httpz.BadRequestResponse(w, err.Error())
		return
	}

	user, found := authmiddleware.ContextGetAuthenticatedUser(r)
	if !found {
		httpz.AuthenticationRequiredResponse(w)
		return
	}

	result, err := h.service.AddCredential(r.Context(), cisco.AddCredential{
		UserID:      new(user.ID()),
		Username:    in.Username,
		Password:    in.Password,
		AuthMethod:  in.AuthMethod,
		Description: in.Description,
	})
	if err != nil {
		h.mapHTTPError(w, r, err)
		return
	}

	type output struct {
		ID          uuid.UUID        `json:"id"`
		Username    string           `json:"username"`
		AuthMethod  cisco.AuthMethod `json:"auth_method"`
		Description *string          `json:"description"`
	}

	err = httpz.WriteJSON(w, http.StatusCreated, httpz.Envelope{"response": output{
		ID:          result.ID,
		Username:    result.Username,
		AuthMethod:  result.AuthMethod,
		Description: result.Description,
	}}, nil)
	if err != nil {
		h.mapHTTPError(w, r, err)
	}
}

func (h *Handler) AddNetworkDevice(w http.ResponseWriter, r *http.Request) {
	type input struct {
		IPAddress    string `json:"ip_address"`
		CredentialID string `json:"credential_id"`
	}

	var in input

	err := httpz.ReadJSON(w, r, &in)
	if err != nil {
		httpz.BadRequestResponse(w, err.Error())
		return
	}

	user, found := authmiddleware.ContextGetAuthenticatedUser(r)
	if !found {
		httpz.AuthenticationRequiredResponse(w)
		return
	}

	credentialID, err := uuid.FromString(in.CredentialID)
	if err != nil {
		httpz.FailedValidationResponse(
			w,
			"one or more fields could not be validated",
			map[string]string{
				"credential_id": "must be a valid UUID",
			},
		)
		return
	}

	result, err := h.service.AddNetworkDevice(r.Context(), cisco.AddNetworkDevice{
		UserID:       user.ID(),
		IPAddress:    in.IPAddress,
		CredentialID: credentialID,
	})
	if err != nil {
		h.mapHTTPError(w, r, err)
		return
	}

	type output struct {
		ID             uuid.UUID        `json:"id"`
		IPAddress      netip.Addr       `json:"ip_address"`
		LastSyncStatus cisco.SyncStatus `json:"last_sync_status"`
	}

	err = httpz.WriteJSON(w, http.StatusCreated, httpz.Envelope{"response": output{
		ID:             result.ID,
		IPAddress:      result.IPAddress,
		LastSyncStatus: result.LastSyncStatus,
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
	case errorz.CodeConflict:
		httpz.ConflictResponse(w, safeErr.UserMsg)

	case errorz.CodeNotFound:
		httpz.NotFoundResponse(w, safeErr.UserMsg)

	case errorz.CodeValidationFailed:
		httpz.FailedValidationResponse(w, safeErr.UserMsg, safeErr.Metadata)

	default:
		h.logger.ErrorContext(r.Context(), "unhandled service error", slog.Any("error", safeErr.Internal))
		httpz.InternalServerErrorResponse(w)
	}
}
