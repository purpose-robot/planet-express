package httpz

import "net/http"

type Envelope = map[string]any

func errorResponse(w http.ResponseWriter, status int, message any) {
	envelope := Envelope{"error": message}

	err := WriteJSON(w, status, envelope, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func ConflictResponse(w http.ResponseWriter, message string) {
	errorResponse(w, http.StatusConflict, message)
}

func NotFoundResponse(w http.ResponseWriter, message string) {
	errorResponse(w, http.StatusNotFound, message)
}

func BadRequestResponse(w http.ResponseWriter, message string) {
	errorResponse(w, http.StatusBadRequest, message)
}

func RateLimitExceededResponse(w http.ResponseWriter) {
	errorResponse(w, http.StatusTooManyRequests, "too many requests, please try again later")
}

func InvalidCredentialsResponse(w http.ResponseWriter, message string) {
	errorResponse(w, http.StatusUnauthorized, message)
}

func InternalServerErrorResponse(w http.ResponseWriter) {
	errorResponse(w, http.StatusInternalServerError, "the server encountered an unexpected condition and could not process your request")
}

func FailedValidationResponse(w http.ResponseWriter, message string, metadata map[string]string) {
	errorResponse(w, http.StatusUnprocessableEntity, Envelope{"message": message, "details": metadata})
}

func AuthenticationRequiredResponse(w http.ResponseWriter) {
	errorResponse(w, http.StatusUnauthorized, "you must be authenticated to access this resource")
}

func InvalidAuthenticationTokenResponse(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	errorResponse(w, http.StatusUnauthorized, "invalid or missing authentication token")
}

func InactiveAccountResponse(w http.ResponseWriter) {
	errorResponse(w, http.StatusForbidden, "your user account must be activated to access this resource")
}

func MissingPermissionResponse(w http.ResponseWriter) {
	errorResponse(w, http.StatusForbidden, "your user doesn't have the required permissions to access this resource")
}
