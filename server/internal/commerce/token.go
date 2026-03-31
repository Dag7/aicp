// Package commerce implements Layer 4 of the AICP protocol: payment token
// issuance, validation, execution, receipt generation, and spending limit enforcement.
package commerce

import (
	"context"
	"fmt"
	"time"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// TokenIssuer creates signed payment tokens (JWS/EdDSA).
type TokenIssuer struct {
	signer *crypto.JWSSigner
	keyID  string
	ttl    time.Duration
}

// NewTokenIssuer creates a new payment token issuer.
func NewTokenIssuer(kp *crypto.KeyPair, ttl time.Duration) (*TokenIssuer, error) {
	did := crypto.PrincipalDID(kp.PublicKey)
	keyID := did + "#key-1"

	signer, err := crypto.NewJWSSigner(kp.PrivateKey, keyID)
	if err != nil {
		return nil, fmt.Errorf("creating JWS signer: %w", err)
	}

	return &TokenIssuer{
		signer: signer,
		keyID:  keyID,
		ttl:    ttl,
	}, nil
}

// Issue creates a signed AICP payment token.
func (ti *TokenIssuer) Issue(ctx context.Context, req aicp.PaymentTokenRequest) (*aicp.PaymentTokenResponse, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(ti.ttl)

	jti := fmt.Sprintf("pt_%s_%d", crypto.SHA256Hex([]byte(req.AgentDID+req.ServiceDID+now.String()))[:16], now.UnixNano())
	nonce := crypto.SHA256Hex([]byte(fmt.Sprintf("%s:%s:%d", req.AgentDID, req.ServiceDID, now.UnixNano())))[:32]
	idempotencyKey := crypto.SHA256Hex([]byte(jti))[:24]

	maxAmount := req.Amount
	if req.MaxAmount != nil {
		maxAmount = *req.MaxAmount
	}

	claims := aicp.PaymentTokenClaims{
		JTI:      jti,
		Issuer:   ti.keyID,
		Subject:  req.AgentDID,
		Audience: req.ServiceDID,
		IssuedAt: now.Unix(),
		ExpiresAt: expiresAt.Unix(),
		PaymentDetails: aicp.PaymentTokenDetails{
			Amount:          req.Amount,
			MaxAmount:       maxAmount,
			Purpose:         req.Purpose,
			ServiceCategory: req.ServiceCategory,
			Nonce:           nonce,
			IdempotencyKey:  idempotencyKey,
		},
	}

	token, err := ti.signer.SignClaims(claims)
	if err != nil {
		return nil, fmt.Errorf("signing payment token: %w", err)
	}

	return &aicp.PaymentTokenResponse{
		Token:     token,
		TokenID:   jti,
		ExpiresAt: expiresAt,
	}, nil
}
