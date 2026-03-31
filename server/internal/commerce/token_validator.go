package commerce

import (
	"context"
	"fmt"
	"time"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// TokenValidator verifies payment tokens: signature, expiry, and replay prevention.
type TokenValidator struct {
	verifier *crypto.JWSVerifier
	backend  ledger.LedgerBackend
}

// NewTokenValidator creates a new payment token validator.
func NewTokenValidator(backend ledger.LedgerBackend) *TokenValidator {
	return &TokenValidator{
		verifier: crypto.NewJWSVerifier(),
		backend:  backend,
	}
}

// ValidationResult holds the outcome of token validation.
type ValidationResult struct {
	Valid    bool                    `json:"valid"`
	Claims   *aicp.PaymentTokenClaims `json:"claims,omitempty"`
	Errors   []string                `json:"errors,omitempty"`
}

// Validate performs full token validation: parse, verify signature, check expiry,
// and check replay.
func (tv *TokenValidator) Validate(ctx context.Context, token string, issuerPubKeyMultibase string) (*ValidationResult, error) {
	result := &ValidationResult{Valid: true}

	// Decode the issuer's public key.
	pubKey, err := crypto.PublicKeyFromMultibase(issuerPubKeyMultibase)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("invalid issuer key: %v", err))
		return result, nil
	}

	// Verify signature and extract claims.
	var claims aicp.PaymentTokenClaims
	if err := tv.verifier.VerifyClaims(token, pubKey, &claims); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("signature verification failed: %v", err))
		return result, nil
	}

	// Check expiry.
	if claims.ExpiresAt > 0 && time.Unix(claims.ExpiresAt, 0).Before(time.Now()) {
		result.Valid = false
		result.Errors = append(result.Errors, "token has expired")
		return result, nil
	}

	// Check replay (JTI uniqueness).
	used, err := tv.backend.IsTokenUsed(ctx, claims.JTI)
	if err != nil {
		return nil, fmt.Errorf("checking token replay: %w", err)
	}
	if used {
		result.Valid = false
		result.Errors = append(result.Errors, "token has already been used (replay)")
		return result, nil
	}

	result.Claims = &claims
	return result, nil
}

// MarkUsed records a token JTI as consumed (replay prevention).
func (tv *TokenValidator) MarkUsed(ctx context.Context, jti string) error {
	return tv.backend.MarkTokenUsed(ctx, jti)
}
