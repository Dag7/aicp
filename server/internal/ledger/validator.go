package ledger

import (
	"context"
	"crypto/ed25519"
	"fmt"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// ValidatorManager handles validator lifecycle operations.
type ValidatorManager struct {
	backend LedgerBackend
}

// NewValidatorManager creates a new validator manager.
func NewValidatorManager(backend LedgerBackend) *ValidatorManager {
	return &ValidatorManager{backend: backend}
}

// RegisterLocal registers the local node as a validator using its key pair.
func (vm *ValidatorManager) RegisterLocal(ctx context.Context, kp *crypto.KeyPair, grpcAddress string) (*aicp.Validator, error) {
	did := crypto.PrincipalDID(kp.PublicKey)
	pubKeyMultibase := crypto.PublicKeyMultibase(kp.PublicKey)

	v := aicp.Validator{
		DID:         did,
		PublicKey:   pubKeyMultibase,
		GRPCAddress: grpcAddress,
		Active:      true,
	}

	if err := vm.backend.RegisterValidator(ctx, v); err != nil {
		return nil, fmt.Errorf("registering validator: %w", err)
	}
	return &v, nil
}

// ValidateSignature verifies that a given signature was produced by the validator
// identified by validatorDID over the given data.
func (vm *ValidatorManager) ValidateSignature(ctx context.Context, validatorDID string, data []byte, signatureB58 string) (bool, error) {
	v, err := vm.backend.GetValidator(ctx, validatorDID)
	if err != nil {
		return false, fmt.Errorf("getting validator: %w", err)
	}

	pubKey, err := crypto.PublicKeyFromMultibase(v.PublicKey)
	if err != nil {
		return false, fmt.Errorf("decoding public key: %w", err)
	}

	return crypto.VerifyJSON(pubKey, data, signatureB58)
}

// GenerateValidatorKeyPair creates a new Ed25519 key pair for a validator node.
func GenerateValidatorKeyPair() (*crypto.KeyPair, error) {
	return crypto.GenerateEd25519KeyPair()
}

// ValidatorDIDFromKey derives the validator DID from an Ed25519 public key.
func ValidatorDIDFromKey(pub ed25519.PublicKey) string {
	return crypto.PrincipalDID(pub)
}
