package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

const (
	contextVCAICP = "https://aicp-protocol.org/credentials/v1"
)

// CredentialManager handles verifiable credential issuance, verification, and storage.
type CredentialManager struct {
	backend ledger.LedgerBackend
	didMgr  *DIDManager
}

// NewCredentialManager creates a new credential manager.
func NewCredentialManager(backend ledger.LedgerBackend, didMgr *DIDManager) *CredentialManager {
	return &CredentialManager{
		backend: backend,
		didMgr:  didMgr,
	}
}

// IssueCredential creates, signs, and stores a verifiable credential.
func (cm *CredentialManager) IssueCredential(ctx context.Context, params IssueParams) (*aicp.VerifiableCredential, error) {
	subjectJSON, err := json.Marshal(params.Subject)
	if err != nil {
		return nil, fmt.Errorf("marshaling subject: %w", err)
	}

	now := time.Now().UTC()
	cred := aicp.VerifiableCredential{
		Context:           []string{aicp.ContextCredentials, contextVCAICP},
		ID:                params.CredentialID,
		Type:              params.Types,
		Issuer:            params.IssuerDID,
		IssuanceDate:      now,
		ExpirationDate:    params.ExpirationDate,
		CredentialSubject: subjectJSON,
		CredentialStatus:  params.Status,
	}

	// Sign the credential.
	proof, err := cm.signCredential(params.IssuerKey, cred)
	if err != nil {
		return nil, fmt.Errorf("signing credential: %w", err)
	}
	cred.Proof = &proof

	// Store on ledger.
	if err := cm.backend.StoreCredential(ctx, cred); err != nil {
		return nil, fmt.Errorf("storing credential: %w", err)
	}

	return &cred, nil
}

// VerifyCredential verifies a credential's signature and revocation status.
func (cm *CredentialManager) VerifyCredential(ctx context.Context, cred aicp.VerifiableCredential) (*VerificationResult, error) {
	result := &VerificationResult{
		CredentialID: cred.ID,
		Valid:        true,
	}

	// Check expiration.
	if cred.ExpirationDate != nil && cred.ExpirationDate.Before(time.Now()) {
		result.Valid = false
		result.Errors = append(result.Errors, "credential has expired")
		return result, nil
	}

	// Verify the proof signature.
	if cred.Proof == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "credential has no proof")
		return result, nil
	}

	issuerResult, err := cm.didMgr.Resolve(ctx, cred.Issuer)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("cannot resolve issuer DID: %v", err))
		return result, nil
	}
	if issuerResult.DocumentMetadata.Deactivated {
		result.Valid = false
		result.Errors = append(result.Errors, "issuer DID is deactivated")
		return result, nil
	}

	pubKey, err := extractPublicKey(issuerResult.Document, cred.Proof.VerificationMethod)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("cannot extract issuer key: %v", err))
		return result, nil
	}

	// Verify signature over credential (without proof field).
	credCopy := cred
	credCopy.Proof = nil
	credBytes, err := canonicalJSON(credCopy)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("cannot serialize credential: %v", err))
		return result, nil
	}

	valid, err := crypto.VerifyJSON(pubKey, credBytes, cred.Proof.ProofValue)
	if err != nil || !valid {
		result.Valid = false
		result.Errors = append(result.Errors, "invalid credential signature")
		return result, nil
	}

	// Check revocation status.
	if cred.CredentialStatus != nil {
		revoked, err := cm.backend.IsCredentialRevoked(ctx, cred.ID)
		if err != nil && err != aicp.ErrNotFound {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("cannot check revocation: %v", err))
			return result, nil
		}
		if revoked {
			result.Valid = false
			result.Errors = append(result.Errors, "credential has been revoked")
			return result, nil
		}
	}

	return result, nil
}

// RevokeCredential revokes a credential by its ID.
func (cm *CredentialManager) RevokeCredential(ctx context.Context, credentialID string, revokerKP *crypto.KeyPair) error {
	cred, err := cm.backend.GetCredential(ctx, credentialID)
	if err != nil {
		return fmt.Errorf("getting credential: %w", err)
	}

	// Only the issuer can revoke.
	revokerDID := crypto.PrincipalDID(revokerKP.PublicKey)
	if cred.Issuer != revokerDID {
		return fmt.Errorf("only the issuer can revoke this credential")
	}

	proof := aicp.Proof{
		Type:               aicp.ProofTypeEd25519Sig2020,
		Created:            time.Now().UTC(),
		VerificationMethod: revokerDID + aicp.PrimaryKeyFragment,
		ProofPurpose:       aicp.ProofPurposeAssertion,
	}

	sigData := []byte("revoke:" + credentialID)
	sigB58, err := crypto.SignJSON(revokerKP.PrivateKey, sigData)
	if err != nil {
		return fmt.Errorf("signing revocation: %w", err)
	}
	proof.ProofValue = sigB58

	return cm.backend.RevokeCredential(ctx, credentialID, proof)
}

// RevokeCredentialByID revokes a credential by its ID using the ledger's
// revocation mechanism. This is used by the HTTP API where the caller's
// authorization is handled at the handler/middleware level.
func (cm *CredentialManager) RevokeCredentialByID(ctx context.Context, credentialID string) error {
	_, err := cm.backend.GetCredential(ctx, credentialID)
	if err != nil {
		return fmt.Errorf("getting credential: %w", err)
	}

	proof := aicp.Proof{
		Type:         aicp.ProofTypeEd25519Sig2020,
		Created:      time.Now().UTC(),
		ProofPurpose: aicp.ProofPurposeAssertion,
	}

	return cm.backend.RevokeCredential(ctx, credentialID, proof)
}

// GetCredential retrieves a credential by ID.
func (cm *CredentialManager) GetCredential(ctx context.Context, credentialID string) (*aicp.VerifiableCredential, error) {
	return cm.backend.GetCredential(ctx, credentialID)
}

func (cm *CredentialManager) signCredential(issuerKey *crypto.KeyPair, cred aicp.VerifiableCredential) (aicp.Proof, error) {
	credBytes, err := canonicalJSON(cred)
	if err != nil {
		return aicp.Proof{}, fmt.Errorf("marshaling credential: %w", err)
	}

	sigB58, err := crypto.SignJSON(issuerKey.PrivateKey, credBytes)
	if err != nil {
		return aicp.Proof{}, fmt.Errorf("signing credential: %w", err)
	}

	issuerDID := crypto.PrincipalDID(issuerKey.PublicKey)
	return aicp.Proof{
		Type:               aicp.ProofTypeEd25519Sig2020,
		Created:            time.Now().UTC(),
		VerificationMethod: issuerDID + aicp.PrimaryKeyFragment,
		ProofPurpose:       aicp.ProofPurposeAssertion,
		ProofValue:         sigB58,
	}, nil
}

// ─── Types ──────────────────────────────────────────────────────────────────

// IssueParams contains the parameters for issuing a verifiable credential.
type IssueParams struct {
	CredentialID   string
	Types          []string
	IssuerDID      string
	IssuerKey      *crypto.KeyPair
	Subject        any
	ExpirationDate *time.Time
	Status         *aicp.CredentialStatus
}

// VerificationResult holds the result of verifying a credential.
type VerificationResult struct {
	CredentialID string   `json:"credential_id"`
	Valid        bool     `json:"valid"`
	Errors       []string `json:"errors,omitempty"`
}
