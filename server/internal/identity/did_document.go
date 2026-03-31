package identity

import (
	"crypto/ed25519"
	"encoding/json"
	"time"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

const (
	// DID Document JSON-LD contexts.
	contextDIDCore = "https://www.w3.org/ns/did/v1"
	contextAICP    = "https://aicp-protocol.org/ns/v1"

	// Verification method type for Ed25519.
	vmTypeEd25519 = "Ed25519VerificationKey2020"
)

// BuildPrincipalDocument creates a DID Document for a principal identity.
func BuildPrincipalDocument(did string, publicKey ed25519.PublicKey) aicp.DIDDocument {
	now := time.Now().UTC()
	keyID := did + "#key-1"

	return aicp.DIDDocument{
		Context:    []string{contextDIDCore, contextAICP},
		ID:         did,
		Controller: did,
		VerificationMethod: []aicp.VerificationMethod{
			{
				ID:                 keyID,
				Type:               vmTypeEd25519,
				Controller:         did,
				PublicKeyMultibase: crypto.PublicKeyMultibase(publicKey),
			},
		},
		Authentication:  []string{keyID},
		AssertionMethod: []string{keyID},
		CapabilityDelegation: []string{keyID},
		Created:    now,
		Updated:    now,
		VersionID:  1,
	}
}

// BuildAgentDocument creates a DID Document for an agent identity.
func BuildAgentDocument(agentDID, principalDID string, agentPublicKey ed25519.PublicKey) aicp.DIDDocument {
	now := time.Now().UTC()
	keyID := agentDID + "#key-1"

	return aicp.DIDDocument{
		Context:    []string{contextDIDCore, contextAICP},
		ID:         agentDID,
		Controller: principalDID,
		VerificationMethod: []aicp.VerificationMethod{
			{
				ID:                 keyID,
				Type:               vmTypeEd25519,
				Controller:         agentDID,
				PublicKeyMultibase: crypto.PublicKeyMultibase(agentPublicKey),
			},
		},
		Authentication:  []string{keyID},
		AssertionMethod: []string{keyID},
		AICPPrincipal:  principalDID,
		Created:         now,
		Updated:         now,
		VersionID:       1,
	}
}

// AddServiceEndpoint adds a service endpoint to a DID Document.
func AddServiceEndpoint(doc *aicp.DIDDocument, id, svcType, endpoint string) {
	doc.Service = append(doc.Service, aicp.ServiceEndpoint{
		ID:              id,
		Type:            svcType,
		ServiceEndpoint: endpoint,
	})
}

// AddVerificationMethod adds an additional verification method to a DID Document.
func AddVerificationMethod(doc *aicp.DIDDocument, keyID string, publicKey ed25519.PublicKey, controller string) {
	doc.VerificationMethod = append(doc.VerificationMethod, aicp.VerificationMethod{
		ID:                 keyID,
		Type:               vmTypeEd25519,
		Controller:         controller,
		PublicKeyMultibase: crypto.PublicKeyMultibase(publicKey),
	})
}

// canonicalJSON produces a deterministic JSON encoding for signing.
func canonicalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}
