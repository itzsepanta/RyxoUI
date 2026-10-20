// Package api exposes the HTTP interface of the panel.
//
// The route table in routes.go is the single source of truth: it wires the
// handlers, declares the scope each handler requires and feeds the OpenAPI
// document.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"
)

// Error codes returned by the API.
const (
	CodeInvalidRequest = "invalid_request"
	CodeUnauthorized   = "unauthorized"
	CodeForbidden      = "forbidden"
	CodeNotFound       = "not_found"
	CodeConflict       = "conflict"
	CodeRateLimited    = "rate_limited"
	CodeUnavailable    = "unavailable"
	CodeInternal       = "internal_error"
)

// ErrorBody is the error envelope.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// errorEnvelope wraps an error payload.
type errorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// listEnvelope wraps a collection.
type listEnvelope struct {
	Data any      `json:"data"`
	Meta listMeta `json:"meta"`
}

// listMeta describes the page that was returned.
type listMeta struct {
	Count  int   `json:"count"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
	Total  int64 `json:"total,omitempty"`
}

// writeJSON writes a JSON document.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(body); err != nil {
		slog.Error("write response", "error", err)
	}
}

// writeError writes an error envelope.
func writeError(w http.ResponseWriter, status int, code, message string, details any) {
	writeJSON(w, status, errorEnvelope{Error: ErrorBody{Code: code, Message: message, Details: details}})
}

// writeList writes a collection envelope. A nil slice is written as an empty
// collection so clients always receive a list and never a null.
func writeList(w http.ResponseWriter, data any, meta listMeta) {
	writeJSON(w, http.StatusOK, listEnvelope{Data: emptyIfNil(data), Meta: meta})
}

// emptyIfNil replaces a nil slice or map with an empty one of the same type.
func emptyIfNil(data any) any {
	if data == nil {
		return []any{}
	}
	value := reflect.ValueOf(data)
	switch value.Kind() {
	case reflect.Slice:
		if value.IsNil() {
			return reflect.MakeSlice(value.Type(), 0, 0).Interface()
		}
	case reflect.Map:
		if value.IsNil() {
			return reflect.MakeMap(value.Type()).Interface()
		}
	}
	return data
}

// apiError is an error with an HTTP status and a stable code.
type apiError struct {
	Status  int
	Code    string
	Message string
	Details any
}

// Error implements the error interface.
func (e *apiError) Error() string { return e.Message }

func errorf(status int, code, format string, args ...any) *apiError {
	return &apiError{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

// badRequest builds a 400 error.
func badRequest(format string, args ...any) *apiError {
	return errorf(http.StatusBadRequest, CodeInvalidRequest, format, args...)
}

// notFound builds a 404 error.
func notFound(format string, args ...any) *apiError {
	return errorf(http.StatusNotFound, CodeNotFound, format, args...)
}

// conflict builds a 409 error.
func conflict(format string, args ...any) *apiError {
	return errorf(http.StatusConflict, CodeConflict, format, args...)
}

// forbidden builds a 403 error.
func forbidden(format string, args ...any) *apiError {
	return errorf(http.StatusForbidden, CodeForbidden, format, args...)
}

// unavailable builds a 503 error.
func unavailable(format string, args ...any) *apiError {
	return errorf(http.StatusServiceUnavailable, CodeUnavailable, format, args...)
}

// handleError maps an error to an HTTP response.
func handleError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		writeError(w, apiErr.Status, apiErr.Code, apiErr.Message, apiErr.Details)
		return
	}
	log.Error("request failed", "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, CodeInternal, "internal server error", nil)
}

// decodeJSON reads a JSON body into target.
func decodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return badRequest("request body is required")
	}
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return badRequest("invalid JSON body: %s", err.Error())
	}
	return nil
}

// queryInt reads an integer query parameter with a default.
func queryInt(r *http.Request, name string, fallback int) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

// queryInt64 reads a 64 bit integer query parameter with a default.
func queryInt64(r *http.Request, name string, fallback int64) int64 {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}
