package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

const (
	// Credential types.
	TypeAgentDelegation       = "AICPAgentDelegation"
	TypeSpendingAuthorization = "AICPSpendingAuthorization"
	TypeServiceAttestation    = "AICPServiceAttestation"
	TypeIdentityVerification  = "AICPIdentityVerification"
)

// IssueDelegationCredential issues an AICPAgentDelegation credential
// from a principal to an agent, defining the agent's delegated capabilities.
func (cm *CredentialManager) IssueDelegationCredential(
	ctx context.Context,
	principalKP *crypto.KeyPair,
	agentDID string,
	capabilities []string,
	constraints aicp.DelegationConstraints,
	expiry *time.Time,
) (*aicp.VerifiableCredential, error) {
	principalDID := crypto.PrincipalDID(principalKP.PublicKey)

	subject := aicp.AgentDelegationSubject{
		ID:                    agentDID,
		Type:                  TypeAgentDelegation,
		Principal:             principalDID,
		DelegatedCapabilities: capabilities,
		Constraints:           constraints,
	}

	credID := fmt.Sprintf("urn:aicp:credential:%s", crypto.SHA256Hex([]byte(principalDID+":"+agentDID+":delegation"))[:16])

	return cm.IssueCredential(ctx, IssueParams{
		CredentialID:   credID,
		Types:          []string{"VerifiableCredential", TypeAgentDelegation},
		IssuerDID:      principalDID,
		IssuerKey:      principalKP,
		Subject:        subject,
		ExpirationDate: expiry,
	})
}

// IssueSpendingAuthorization issues an AICPSpendingAuthorization credential
// that defines an agent's payment capabilities and limits.
func (cm *CredentialManager) IssueSpendingAuthorization(
	ctx context.Context,
	principalKP *crypto.KeyPair,
	agentDID string,
	paymentMethods []aicp.PaymentMethodRef,
	limits aicp.SpendingLimits,
	expiry *time.Time,
) (*aicp.VerifiableCredential, error) {
	principalDID := crypto.PrincipalDID(principalKP.PublicKey)

	subject := aicp.SpendingAuthorizationSubject{
		ID:             agentDID,
		Type:           TypeSpendingAuthorization,
		PaymentMethods: paymentMethods,
		Limits:         limits,
	}

	credID := fmt.Sprintf("urn:aicp:credential:%s", crypto.SHA256Hex([]byte(principalDID+":"+agentDID+":spending"))[:16])

	return cm.IssueCredential(ctx, IssueParams{
		CredentialID:   credID,
		Types:          []string{"VerifiableCredential", TypeSpendingAuthorization},
		IssuerDID:      principalDID,
		IssuerKey:      principalKP,
		Subject:        subject,
		ExpirationDate: expiry,
	})
}

// ServiceAttestationSubject is the credentialSubject for a service attestation.
type ServiceAttestationSubject struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	ServiceName string   `json:"serviceName"`
	ServiceDID  string   `json:"serviceDid"`
	Categories  []string `json:"categories"`
	Attestation string   `json:"attestation"`
}

// IssueServiceAttestation issues an AICPServiceAttestation credential
// attesting to a service's legitimacy or quality.
func (cm *CredentialManager) IssueServiceAttestation(
	ctx context.Context,
	attesterKP *crypto.KeyPair,
	serviceDID string,
	serviceName string,
	categories []string,
	attestation string,
	expiry *time.Time,
) (*aicp.VerifiableCredential, error) {
	attesterDID := crypto.PrincipalDID(attesterKP.PublicKey)

	subject := ServiceAttestationSubject{
		ID:          serviceDID,
		Type:        TypeServiceAttestation,
		ServiceName: serviceName,
		ServiceDID:  serviceDID,
		Categories:  categories,
		Attestation: attestation,
	}

	credID := fmt.Sprintf("urn:aicp:credential:%s", crypto.SHA256Hex([]byte(attesterDID+":"+serviceDID+":attestation"))[:16])

	return cm.IssueCredential(ctx, IssueParams{
		CredentialID:   credID,
		Types:          []string{"VerifiableCredential", TypeServiceAttestation},
		IssuerDID:      attesterDID,
		IssuerKey:      attesterKP,
		Subject:        subject,
		ExpirationDate: expiry,
	})
}
