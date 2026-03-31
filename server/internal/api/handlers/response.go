package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// Response is the standardized JSON response envelope.
type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error is the standardized error response.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// JSON writes a success response.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    data,
	})
}

// JSONError writes an error response.
func JSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Success: false,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	})
}

// JSONErrorDetail writes an error response with additional detail.
func JSONErrorDetail(w http.ResponseWriter, status int, code, message, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Success: false,
		Error: &Error{
			Code:    code,
			Message: message,
			Detail:  detail,
		},
	})
}

// ProtocolErrorResponse maps a protocol error to an HTTP response.
func ProtocolErrorResponse(w http.ResponseWriter, err error) {
	pe, ok := aicp.IsProtocolError(err)
	if !ok {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	JSONErrorDetail(w, pe.StatusCode, pe.Code, pe.Message, pe.Detail)
}

// DecodeJSON decodes the request body into the given destination.
func DecodeJSON(r *http.Request, dest any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dest)
}
