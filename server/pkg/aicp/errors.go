package aicp

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors for protocol-level failures.
var (
	ErrNotFound              = errors.New("resource not found")
	ErrAlreadyExists         = errors.New("resource already exists")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrForbidden             = errors.New("forbidden")
	ErrInvalidInput          = errors.New("invalid input")
	ErrRateLimited           = errors.New("rate limited")
	ErrCredentialRevoked     = errors.New("credential revoked")
	ErrCredentialExpired     = errors.New("credential expired")
	ErrDIDDeactivated        = errors.New("DID deactivated")
	ErrPaymentFailed         = errors.New("payment failed")
	ErrPaymentTokenExpired   = errors.New("payment token expired")
	ErrPaymentTokenReplay    = errors.New("payment token already used")
	ErrOnboardingFailed      = errors.New("onboarding failed")
	ErrInvalidPhase          = errors.New("invalid onboarding phase transition")
	ErrSessionExpired        = errors.New("onboarding session expired")
	ErrConsensusFailure      = errors.New("consensus failure")
	ErrValidatorNotFound     = errors.New("validator not found")
	ErrInsufficientSigs      = errors.New("insufficient validator signatures")
	ErrSpendingLimitExceeded = errors.New("spending limit exceeded")
	ErrServiceUnavailable    = errors.New("service unavailable")
	ErrCapabilityDenied      = errors.New("capability denied")
	ErrApprovalRequired      = errors.New("approval required")
	ErrApprovalExpired       = errors.New("approval expired")
	ErrApprovalDenied        = errors.New("approval denied")
	ErrTokenAlreadyUsed      = errors.New("payment token already used")
)

// ProtocolError represents a structured error returned to API clients.
// It includes an HTTP status code, a machine-readable error code, and
// a human-readable message.
type ProtocolError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Detail     string `json:"detail,omitempty"`
}

// Error implements the error interface.
func (e *ProtocolError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns nil because ProtocolError is a leaf error.
func (e *ProtocolError) Unwrap() error { return nil }

// NewProtocolError creates a new ProtocolError.
func NewProtocolError(statusCode int, code, message string) *ProtocolError {
	return &ProtocolError{
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
	}
}

// WithDetail returns a copy of the error with additional detail.
func (e *ProtocolError) WithDetail(detail string) *ProtocolError {
	return &ProtocolError{
		StatusCode: e.StatusCode,
		Code:       e.Code,
		Message:    e.Message,
		Detail:     detail,
	}
}

// Pre-defined protocol errors for common cases.
var (
	ErrCodeNotFound          = NewProtocolError(http.StatusNotFound, "NOT_FOUND", "the requested resource was not found")
	ErrCodeBadRequest        = NewProtocolError(http.StatusBadRequest, "BAD_REQUEST", "the request is malformed or invalid")
	ErrCodeUnauthorized      = NewProtocolError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication is required")
	ErrCodeForbidden         = NewProtocolError(http.StatusForbidden, "FORBIDDEN", "you do not have permission to perform this action")
	ErrCodeConflict          = NewProtocolError(http.StatusConflict, "CONFLICT", "the resource already exists")
	ErrCodeRateLimited       = NewProtocolError(http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
	ErrCodeInternal          = NewProtocolError(http.StatusInternalServerError, "INTERNAL_ERROR", "an internal error occurred")
	ErrCodeDIDNotFound       = NewProtocolError(http.StatusNotFound, "DID_NOT_FOUND", "the specified DID was not found")
	ErrCodeDIDDeactivated    = NewProtocolError(http.StatusGone, "DID_DEACTIVATED", "the specified DID has been deactivated")
	ErrCodeCredentialRevoked = NewProtocolError(http.StatusForbidden, "CREDENTIAL_REVOKED", "the presented credential has been revoked")
	ErrCodeCredentialExpired = NewProtocolError(http.StatusForbidden, "CREDENTIAL_EXPIRED", "the presented credential has expired")
	ErrCodeTokenExpired      = NewProtocolError(http.StatusForbidden, "PAYMENT_TOKEN_EXPIRED", "the payment token has expired")
	ErrCodeTokenReplay       = NewProtocolError(http.StatusConflict, "PAYMENT_TOKEN_REPLAY", "the payment token has already been used")
	ErrCodeSpendingLimit     = NewProtocolError(http.StatusForbidden, "SPENDING_LIMIT_EXCEEDED", "the transaction exceeds the agent's spending limit")
	ErrCodeInvalidPhase      = NewProtocolError(http.StatusBadRequest, "INVALID_PHASE", "invalid onboarding phase transition")
	ErrCodeSessionExpired    = NewProtocolError(http.StatusGone, "SESSION_EXPIRED", "the onboarding session has expired")
	ErrCodePaymentFailed     = NewProtocolError(http.StatusPaymentRequired, "PAYMENT_FAILED", "payment processing failed")
	ErrCodeCapabilityDenied  = NewProtocolError(http.StatusForbidden, "CAPABILITY_DENIED", "the agent does not have the required capability")
	ErrCodeApprovalRequired  = NewProtocolError(http.StatusAccepted, "APPROVAL_REQUIRED", "the transaction requires principal approval")
	ErrCodeApprovalExpired   = NewProtocolError(http.StatusGone, "APPROVAL_EXPIRED", "the approval request has expired")
	ErrCodeApprovalDenied    = NewProtocolError(http.StatusForbidden, "APPROVAL_DENIED", "the principal denied the transaction")
	ErrCodeApprovalNotFound  = NewProtocolError(http.StatusNotFound, "APPROVAL_NOT_FOUND", "the approval request was not found")
)

// IsProtocolError checks whether an error is a ProtocolError and returns it.
func IsProtocolError(err error) (*ProtocolError, bool) {
	var pe *ProtocolError
	if errors.As(err, &pe) {
		return pe, true
	}
	return nil, false
}
