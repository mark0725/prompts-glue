package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

type Response struct {
	Success   bool       `json:"success"`
	Data      any        `json:"data,omitempty"`
	Error     *ErrorBody `json:"error,omitempty"`
	RequestID string     `json:"request_id"`
}

type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func RequestID(r *http.Request) string {
	if value := r.Context().Value(requestIDKey{}); value != nil {
		if id, ok := value.(string); ok {
			return id
		}
	}
	if id := r.Header.Get("X-Request-Id"); id != "" {
		return id
	}
	return uuid.NewString()
}

func WithRequestID(r *http.Request, requestID string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), requestIDKey{}, requestID))
}

func WriteJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response{Success: true, Data: data, RequestID: RequestID(r)})
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code string, err error) {
	if err == nil {
		err = errors.New(http.StatusText(status))
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response{Success: false, Error: &ErrorBody{Code: code, Message: err.Error()}, RequestID: RequestID(r)})
}

func DecodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

type requestIDKey struct{}
