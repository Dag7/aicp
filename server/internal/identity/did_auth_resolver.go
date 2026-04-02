package identity

import (
	"context"
	"crypto/ed25519"
	"fmt"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
)

// DIDAuthResolver resolves DID public keys for JWS signature verification
// in the DID-Auth middleware. It implements middleware.DIDKeyResolver.
type DIDAuthResolver struct {
	didMgr *DIDManager
}

// NewDIDAuthResolver creates a new resolver that looks up public keys from DID documents.
func NewDIDAuthResolver(didMgr *DIDManager) *DIDAuthResolver {
	return &DIDAuthResolver{didMgr: didMgr}
}

// ResolvePublicKey resolves the Ed25519 public key for a DID, matching by keyID if possible.
// Returns an error if the DID is not found, is deactivated, or has no verification methods.
func (r *DIDAuthResolver) ResolvePublicKey(ctx context.Context, did string, keyID string) (ed25519.PublicKey, error) {
	result, err := r.didMgr.Resolve(ctx, did)
	if err != nil {
		return nil, fmt.Errorf("resolving DID %s: %w", did, err)
	}

	if result.DocumentMetadata.Deactivated {
		return nil, fmt.Errorf("DID %s is deactivated", did)
	}

	if len(result.Document.VerificationMethod) == 0 {
		return nil, fmt.Errorf("DID %s has no verification methods", did)
	}

	// Try to match the exact keyID from the kid header.
	for _, vm := range result.Document.VerificationMethod {
		if vm.ID == keyID {
			return crypto.PublicKeyFromMultibase(vm.PublicKeyMultibase)
		}
	}

	// Fall back to the primary verification method (first one).
	return crypto.PublicKeyFromMultibase(result.Document.VerificationMethod[0].PublicKeyMultibase)
}
