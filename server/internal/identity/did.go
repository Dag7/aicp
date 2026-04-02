// Package identity implements Layer 1 of the AICP protocol: DID lifecycle
// management, verifiable credential issuance/verification, delegation chains,
// and credential revocation.
package identity

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// DIDManager manages the full lifecycle of did:aicp DIDs: creation, resolution,
// update, and deactivation.
type DIDManager struct {
	backend ledger.LedgerBackend
}

// NewDIDManager creates a new DID manager.
func NewDIDManager(backend ledger.LedgerBackend) *DIDManager {
	return &DIDManager{backend: backend}
}

// CreatePrincipal registers a new principal DID on the ledger.
func (dm *DIDManager) CreatePrincipal(ctx context.Context, kp *crypto.KeyPair) (*aicp.DIDDocument, *aicp.AnchorReceipt, error) {
	did := crypto.PrincipalDID(kp.PublicKey)

	doc := BuildPrincipalDocument(did, kp.PublicKey)

	proof, err := dm.createProof(kp, doc)
	if err != nil {
		return nil, nil, fmt.Errorf("creating proof: %w", err)
	}

	receipt, err := dm.backend.AnchorDID(ctx, did, doc, proof)
	if err != nil {
		return nil, nil, fmt.Errorf("anchoring principal DID: %w", err)
	}

	return &doc, receipt, nil
}

// CreateAgent registers a new agent DID under a principal's control.
func (dm *DIDManager) CreateAgent(ctx context.Context, principalKP *crypto.KeyPair, agentKP *crypto.KeyPair) (*aicp.DIDDocument, *aicp.AnchorReceipt, error) {
	principalDID := crypto.PrincipalDID(principalKP.PublicKey)

	// Verify principal exists and is active.
	result, err := dm.backend.ResolveDID(ctx, principalDID)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving principal DID: %w", err)
	}
	if result.DocumentMetadata.Deactivated {
		return nil, nil, fmt.Errorf("principal DID is deactivated")
	}

	agentDID := crypto.AgentDID(principalKP.PublicKey, agentKP.PublicKey)
	doc := BuildAgentDocument(agentDID, principalDID, agentKP.PublicKey)

	proof, err := dm.createProof(principalKP, doc)
	if err != nil {
		return nil, nil, fmt.Errorf("creating proof: %w", err)
	}

	receipt, err := dm.backend.AnchorDID(ctx, agentDID, doc, proof)
	if err != nil {
		return nil, nil, fmt.Errorf("anchoring agent DID: %w", err)
	}

	return &doc, receipt, nil
}

// Resolve retrieves the DID document and metadata for a given DID.
func (dm *DIDManager) Resolve(ctx context.Context, did string) (*aicp.DIDResolutionResult, error) {
	result, err := dm.backend.ResolveDID(ctx, did)
	if err != nil {
		return nil, fmt.Errorf("resolving DID: %w", err)
	}
	return result, nil
}

// Update modifies an existing DID document with a new version.
func (dm *DIDManager) Update(ctx context.Context, did string, doc aicp.DIDDocument, signerKP *crypto.KeyPair) (*aicp.AnchorReceipt, error) {
	// Verify the signer is authorized (controller of the DID).
	existing, err := dm.backend.ResolveDID(ctx, did)
	if err != nil {
		return nil, fmt.Errorf("resolving existing DID: %w", err)
	}
	if existing.DocumentMetadata.Deactivated {
		return nil, fmt.Errorf("DID is deactivated")
	}

	signerDID := crypto.PrincipalDID(signerKP.PublicKey)
	if existing.Document.Controller != signerDID && existing.Document.ID != signerDID {
		return nil, fmt.Errorf("signer %s is not authorized to update DID %s", signerDID, did)
	}

	doc.VersionID = existing.DocumentMetadata.VersionID + 1
	doc.Updated = time.Now().UTC()

	proof, err := dm.createProof(signerKP, doc)
	if err != nil {
		return nil, fmt.Errorf("creating proof: %w", err)
	}

	return dm.backend.UpdateDIDDocument(ctx, did, doc, proof)
}

// Deactivate permanently deactivates a DID.
func (dm *DIDManager) Deactivate(ctx context.Context, did string, signerKP *crypto.KeyPair) error {
	existing, err := dm.backend.ResolveDID(ctx, did)
	if err != nil {
		return fmt.Errorf("resolving DID: %w", err)
	}
	if existing.DocumentMetadata.Deactivated {
		return fmt.Errorf("DID is already deactivated")
	}

	signerDID := crypto.PrincipalDID(signerKP.PublicKey)
	if existing.Document.Controller != signerDID && existing.Document.ID != signerDID {
		return fmt.Errorf("signer %s is not authorized to deactivate DID %s", signerDID, did)
	}

	proof, err := dm.createDeactivationProof(signerKP, did)
	if err != nil {
		return fmt.Errorf("creating deactivation proof: %w", err)
	}

	return dm.backend.DeactivateDID(ctx, did, proof)
}

// AnchorAgent anchors an agent DID document to the ledger using a pre-built proof.
// Used by the HTTP handler when the caller's authentication is handled at the middleware level.
func (dm *DIDManager) AnchorAgent(ctx context.Context, agentDID string, doc aicp.DIDDocument, proof aicp.Proof) (*aicp.AnchorReceipt, error) {
	return dm.backend.AnchorDID(ctx, agentDID, doc, proof)
}

// UpdateDocument persists an updated DID document to the ledger.
// Used by the HTTP handler when the caller's authorization is verified at the handler level.
func (dm *DIDManager) UpdateDocument(ctx context.Context, did string, doc aicp.DIDDocument, proof aicp.Proof) (*aicp.AnchorReceipt, error) {
	return dm.backend.UpdateDIDDocument(ctx, did, doc, proof)
}

// DeactivateWithProof deactivates a DID on the ledger using a pre-built proof.
// Used by the HTTP handler when the caller's authorization is verified at the handler level.
func (dm *DIDManager) DeactivateWithProof(ctx context.Context, did string, proof aicp.Proof) error {
	return dm.backend.DeactivateDID(ctx, did, proof)
}

// ServerAttestationProof creates a proof that records a server-attested operation,
// used when the DID-Auth middleware has already verified the caller's identity.
func ServerAttestationProof(callerDID string) aicp.Proof {
	return aicp.Proof{
		Type:               aicp.ProofTypeEd25519Sig2020,
		Created:            time.Now().UTC(),
		VerificationMethod: callerDID + aicp.PrimaryKeyFragment,
		ProofPurpose:       aicp.ProofPurposeAssertion,
	}
}

// ListAgents returns all agent DIDs controlled by the given principal.
func (dm *DIDManager) ListAgents(ctx context.Context, principalDID string) ([]string, error) {
	return dm.backend.ListAgentDIDs(ctx, principalDID)
}

// VerifyDIDAuth verifies a DID-Auth challenge-response proof.
func (dm *DIDManager) VerifyDIDAuth(ctx context.Context, did string, challenge string, proof aicp.Proof) (bool, error) {
	result, err := dm.backend.ResolveDID(ctx, did)
	if err != nil {
		return false, fmt.Errorf("resolving DID: %w", err)
	}
	if result.DocumentMetadata.Deactivated {
		return false, fmt.Errorf("DID is deactivated")
	}

	// Find the verification method referenced in the proof.
	pubKey, err := extractPublicKey(result.Document, proof.VerificationMethod)
	if err != nil {
		return false, fmt.Errorf("extracting public key: %w", err)
	}

	// Verify the signature over the challenge.
	return crypto.VerifyJSON(pubKey, []byte(challenge), proof.ProofValue)
}

// createProof signs a DID document and returns a Proof object.
func (dm *DIDManager) createProof(kp *crypto.KeyPair, doc aicp.DIDDocument) (aicp.Proof, error) {
	docBytes, err := canonicalJSON(doc)
	if err != nil {
		return aicp.Proof{}, fmt.Errorf("marshaling document: %w", err)
	}

	sigB58, err := crypto.SignJSON(kp.PrivateKey, docBytes)
	if err != nil {
		return aicp.Proof{}, fmt.Errorf("signing document: %w", err)
	}

	did := crypto.PrincipalDID(kp.PublicKey)
	return aicp.Proof{
		Type:               aicp.ProofTypeEd25519Sig2020,
		Created:            time.Now().UTC(),
		VerificationMethod: did + aicp.PrimaryKeyFragment,
		ProofPurpose:       aicp.ProofPurposeAssertion,
		ProofValue:         sigB58,
	}, nil
}

func (dm *DIDManager) createDeactivationProof(kp *crypto.KeyPair, did string) (aicp.Proof, error) {
	data := []byte("deactivate:" + did)
	sigB58, err := crypto.SignJSON(kp.PrivateKey, data)
	if err != nil {
		return aicp.Proof{}, fmt.Errorf("signing deactivation: %w", err)
	}

	signerDID := crypto.PrincipalDID(kp.PublicKey)
	return aicp.Proof{
		Type:               aicp.ProofTypeEd25519Sig2020,
		Created:            time.Now().UTC(),
		VerificationMethod: signerDID + aicp.PrimaryKeyFragment,
		ProofPurpose:       aicp.ProofPurposeAssertion,
		ProofValue:         sigB58,
	}, nil
}

// extractPublicKey finds and decodes the public key from a DID document
// for a given verification method ID.
func extractPublicKey(doc *aicp.DIDDocument, vmID string) (ed25519.PublicKey, error) {
	for _, vm := range doc.VerificationMethod {
		if vm.ID == vmID {
			return crypto.PublicKeyFromMultibase(vm.PublicKeyMultibase)
		}
	}
	return nil, fmt.Errorf("verification method %s not found in DID document", vmID)
}
