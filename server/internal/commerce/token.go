// Package commerce implements Layer 4 of the AICP protocol: payment token
// issuance, validation, execution, receipt generation, and spending limit enforcement.
package commerce

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	aicpcrypto "github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// supportedCurrencies is the allowlist of accepted ISO 4217 + stablecoin codes.
var supportedCurrencies = map[string]bool{
	"USD": true, "EUR": true, "GBP": true, "JPY": true,
	"CAD": true, "AUD": true, "CHF": true, "CNY": true,
	"USDC": true, "USDT": true,
}

// maxPaymentAmount is the absolute ceiling for any single payment amount.
var maxPaymentAmount = new(big.Float).SetFloat64(999_999_999.99)

// TokenIssuer creates signed payment tokens (JWS/EdDSA).
type TokenIssuer struct {
	signer *aicpcrypto.JWSSigner
	keyID  string
	ttl    time.Duration
}

// NewTokenIssuer creates a new payment token issuer.
func NewTokenIssuer(kp *aicpcrypto.KeyPair, ttl time.Duration) (*TokenIssuer, error) {
	did := aicpcrypto.PrincipalDID(kp.PublicKey)
	keyID := did + aicp.PrimaryKeyFragment

	signer, err := aicpcrypto.NewJWSSigner(kp.PrivateKey, keyID)
	if err != nil {
		return nil, fmt.Errorf("creating JWS signer: %w", err)
	}

	return &TokenIssuer{
		signer: signer,
		keyID:  keyID,
		ttl:    ttl,
	}, nil
}

// validateAmount checks that a payment amount is positive and within bounds.
func validateAmount(amount aicp.MoneyAmount) error {
	val, ok := new(big.Float).SetString(amount.Value)
	if !ok {
		return fmt.Errorf("invalid amount value")
	}
	if val.Sign() <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if val.Cmp(maxPaymentAmount) > 0 {
		return fmt.Errorf("amount exceeds maximum")
	}
	return nil
}

// cryptoRandomHex generates n random bytes and returns them as a hex string.
func cryptoRandomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// Issue creates a signed AICP payment token.
func (ti *TokenIssuer) Issue(ctx context.Context, req aicp.PaymentTokenRequest) (*aicp.PaymentTokenResponse, error) {
	// Validate amount at the commerce layer (defense in depth).
	if err := validateAmount(req.Amount); err != nil {
		return nil, fmt.Errorf("invalid payment amount: %w", err)
	}

	// Validate currency against allowlist.
	if !supportedCurrencies[req.Amount.Currency] {
		return nil, fmt.Errorf("unsupported currency: %s", req.Amount.Currency)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(ti.ttl)

	// Use cryptographic randomness for JTI and nonce to prevent collisions.
	jti := fmt.Sprintf("pt_%s", cryptoRandomHex(16))
	nonce := cryptoRandomHex(16)
	idempotencyKey := aicpcrypto.SHA256Hex([]byte(jti))[:32]

	maxAmount := req.Amount
	if req.MaxAmount != nil {
		if err := validateAmount(*req.MaxAmount); err != nil {
			return nil, fmt.Errorf("invalid max amount: %w", err)
		}
		maxAmount = *req.MaxAmount
	}

	claims := aicp.PaymentTokenClaims{
		JTI:       jti,
		Issuer:    ti.keyID,
		Subject:   req.AgentDID,
		Audience:  req.ServiceDID,
		IssuedAt:  now.Unix(),
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
