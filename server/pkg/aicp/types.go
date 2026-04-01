package aicp

import (
	"encoding/json"
	"strconv"
	"time"
)

// ─── DID Types ──────────────────────────────────────────────────────────────

// DIDDocument represents a W3C DID Document for the did:aicp method.
type DIDDocument struct {
	Context              []string              `json:"@context"`
	ID                   string                `json:"id"`
	Controller           string                `json:"controller"`
	VerificationMethod   []VerificationMethod  `json:"verificationMethod"`
	Authentication       []string              `json:"authentication"`
	AssertionMethod      []string              `json:"assertionMethod,omitempty"`
	CapabilityDelegation []string              `json:"capabilityDelegation,omitempty"`
	Service              []ServiceEndpoint     `json:"service,omitempty"`
	AICPPrincipal        string                `json:"aicp:principal,omitempty"`
	AICPDelegationCred   string                `json:"aicp:delegationCredential,omitempty"`
	Created              time.Time             `json:"created"`
	Updated              time.Time             `json:"updated"`
	VersionID            int                   `json:"versionId"`
}

// VerificationMethod represents a public key in a DID Document.
type VerificationMethod struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Controller         string `json:"controller"`
	PublicKeyMultibase string `json:"publicKeyMultibase"`
}

// ServiceEndpoint represents a service in a DID Document.
type ServiceEndpoint struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	ServiceEndpoint string `json:"serviceEndpoint"`
}

// DIDDocumentMetadata holds ledger-specific metadata for a resolved DID.
type DIDDocumentMetadata struct {
	Created       time.Time `json:"created"`
	Updated       time.Time `json:"updated"`
	VersionID     int       `json:"versionId"`
	Deactivated   bool      `json:"deactivated"`
	NextUpdate    string    `json:"nextUpdate,omitempty"`
	DocumentHash  string    `json:"documentHash"`
}

// DIDResolutionResult is the full result of resolving a DID.
type DIDResolutionResult struct {
	Document         *DIDDocument         `json:"didDocument"`
	DocumentMetadata *DIDDocumentMetadata `json:"didDocumentMetadata"`
}

// ─── Verifiable Credential Types ────────────────────────────────────────────

// VerifiableCredential represents a W3C Verifiable Credential.
type VerifiableCredential struct {
	Context           []string               `json:"@context"`
	ID                string                 `json:"id"`
	Type              []string               `json:"type"`
	Issuer            string                 `json:"issuer"`
	IssuanceDate      time.Time              `json:"issuanceDate"`
	ExpirationDate    *time.Time             `json:"expirationDate,omitempty"`
	CredentialSubject json.RawMessage        `json:"credentialSubject"`
	CredentialStatus  *CredentialStatus      `json:"credentialStatus,omitempty"`
	Proof             *Proof                 `json:"proof,omitempty"`
}

// CredentialStatus references the revocation mechanism for a credential.
type CredentialStatus struct {
	ID                       string `json:"id"`
	Type                     string `json:"type"`
	RevocationListIndex      string `json:"revocationListIndex"`
	RevocationListCredential string `json:"revocationListCredential"`
}

// Proof holds a cryptographic proof (signature) over a document.
type Proof struct {
	Type               string    `json:"type"`
	Created            time.Time `json:"created"`
	VerificationMethod string    `json:"verificationMethod"`
	ProofPurpose       string    `json:"proofPurpose"`
	ProofValue         string    `json:"proofValue"`
	Challenge          string    `json:"challenge,omitempty"`
	Domain             string    `json:"domain,omitempty"`
}

// VerifiablePresentation wraps one or more VCs with a holder proof.
type VerifiablePresentation struct {
	Context              []string               `json:"@context"`
	Type                 []string               `json:"type"`
	Holder               string                 `json:"holder"`
	VerifiableCredential []VerifiableCredential `json:"verifiableCredential"`
	Proof                *Proof                 `json:"proof,omitempty"`
}

// ─── Agent Credential Subject Types ─────────────────────────────────────────

// AgentDelegationSubject is the credentialSubject of an AICPAgentDelegation VC.
type AgentDelegationSubject struct {
	ID                    string              `json:"id"`
	Type                  string              `json:"type"`
	Principal             string              `json:"principal"`
	DelegatedCapabilities []string            `json:"delegatedCapabilities"`
	Constraints           DelegationConstraints `json:"constraints"`
}

// DelegationConstraints defines operational limits for an agent.
type DelegationConstraints struct {
	MaxTransactionAmount    MoneyAmount `json:"maxTransactionAmount"`
	DailySpendingLimit      MoneyAmount `json:"dailySpendingLimit"`
	MonthlySpendingLimit    MoneyAmount `json:"monthlySpendingLimit"`
	AllowedServiceCategories []string   `json:"allowedServiceCategories"`
	AllowedGeographies      []string   `json:"allowedGeographies,omitempty"`
	MaxConcurrentSubs       int        `json:"maxConcurrentSubscriptions,omitempty"`
}

// MoneyAmount represents a currency amount.
type MoneyAmount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

// SpendingAuthorizationSubject is the credentialSubject of an AICPSpendingAuthorization VC.
type SpendingAuthorizationSubject struct {
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	PaymentMethods []PaymentMethodRef `json:"paymentMethods"`
	Limits         SpendingLimits  `json:"limits"`
	AllowedCategories []string     `json:"allowedMerchantCategories,omitempty"`
	RequiresApprovalAbove *MoneyAmount `json:"requiresApprovalAbove,omitempty"`
}

// PaymentMethodRef references a configured payment adapter.
type PaymentMethodRef struct {
	Type         string   `json:"type"`
	Provider     string   `json:"provider"`
	AdapterID    string   `json:"adapterId"`
	Capabilities []string `json:"capabilities"`
}

// SpendingLimits defines per-transaction, daily, and monthly limits.
type SpendingLimits struct {
	PerTransaction MoneyAmount `json:"perTransaction"`
	Daily          MoneyAmount `json:"daily"`
	Monthly        MoneyAmount `json:"monthly"`
}

// ─── ANS (Agent Name System) Types ──────────────────────────────────────────

// ANSRecord is the service discovery record published at /.well-known/aicp.json.
type ANSRecord struct {
	AICPVersion    string              `json:"aicp_version"`
	Service        ANSServiceInfo      `json:"service"`
	Onboarding     ANSOnboardingInfo   `json:"onboarding"`
	Authentication ANSAuthInfo         `json:"authentication"`
	Pricing        *ANSPricingInfo     `json:"pricing,omitempty"`
	DiscoveryTags  []string            `json:"discovery_tags,omitempty"`
	Trust          *ANSTrustInfo       `json:"trust,omitempty"`
	Endpoints      *ANSEndpoints       `json:"endpoints,omitempty"`
}

// ANSServiceInfo describes the service.
type ANSServiceInfo struct {
	Name             string   `json:"name"`
	DID              string   `json:"did"`
	Description      string   `json:"description,omitempty"`
	Category         []string `json:"category"`
	Homepage         string   `json:"homepage,omitempty"`
	LogoURL          string   `json:"logoUrl,omitempty"`
	TOSURL           string   `json:"tosUrl,omitempty"`
	PrivacyPolicyURL string   `json:"privacyPolicyUrl,omitempty"`
	SupportContact   string   `json:"supportContact,omitempty"`
	APIDocsURL       string   `json:"apiDocsUrl,omitempty"`
}

// ANSOnboardingInfo describes the onboarding requirements.
type ANSOnboardingInfo struct {
	Flows                  []string `json:"flows"`
	RequiredCredentials    []string `json:"required_credentials"`
	OptionalCredentials    []string `json:"optional_credentials,omitempty"`
	AcceptedPayments       []string `json:"accepted_payments,omitempty"`
	MinimumTrustScore      int      `json:"minimum_trust_score,omitempty"`
	ComplianceRequirements []string `json:"compliance_requirements,omitempty"`
}

// ANSAuthInfo describes authentication requirements.
type ANSAuthInfo struct {
	DIDMethods    []string `json:"did_methods"`
	AuthProtocols []string `json:"auth_protocols"`
	SessionTTL    int      `json:"session_ttl_seconds,omitempty"`
}

// ANSPricingInfo describes service pricing.
type ANSPricingInfo struct {
	Model    string           `json:"model"`
	Currency string           `json:"currency,omitempty"`
	Tiers    []ANSPricingTier `json:"tiers,omitempty"`
}

// ANSPricingTier represents a pricing tier.
type ANSPricingTier struct {
	Name        string `json:"name"`
	Rate        string `json:"rate,omitempty"`
	Description string `json:"description,omitempty"`
}

// ANSTrustInfo describes the service's trust signals.
type ANSTrustInfo struct {
	VerifiedBy       []string `json:"verified_by,omitempty"`
	EstablishedSince string   `json:"established_since,omitempty"`
	Certifications   []string `json:"certifications,omitempty"`
}

// ANSEndpoints lists custom endpoint URLs.
type ANSEndpoints struct {
	Onboard   string `json:"onboard,omitempty"`
	APIBase   string `json:"api_base,omitempty"`
	WebSocket string `json:"websocket,omitempty"`
	Health    string `json:"health,omitempty"`
}

// ─── Onboarding Types ───────────────────────────────────────────────────────

// OnboardingPhase represents the current phase of an onboarding flow.
type OnboardingPhase string

const (
	PhaseInitiated              OnboardingPhase = "initiated"
	PhaseIdentityPresented      OnboardingPhase = "identity_presented"
	PhaseCapabilitiesNegotiated OnboardingPhase = "capabilities_negotiated"
	PhasePaymentAuthorized      OnboardingPhase = "payment_authorized"
	PhaseProvisioned            OnboardingPhase = "provisioned"
	PhaseFailed                 OnboardingPhase = "failed"
)

// OnboardingState is the full state of an onboarding session.
type OnboardingState struct {
	SessionID    string           `json:"session_id"`
	Phase        OnboardingPhase  `json:"phase"`
	AgentDID     string           `json:"agent_did"`
	ServiceDID   string           `json:"service_did"`
	StartedAt    time.Time        `json:"started_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	ExpiresAt    time.Time        `json:"expires_at"`
	FailedReason string           `json:"failed_reason,omitempty"`
}

// OnboardInitRequest is the body of POST /aicp/v1/onboard.
type OnboardInitRequest struct {
	AgentDID              string                  `json:"agent_did"`
	Presentation          VerifiablePresentation  `json:"presentation"`
	RequestedCapabilities []string                `json:"requested_capabilities"`
}

// OnboardInitResponse is returned from POST /aicp/v1/onboard.
type OnboardInitResponse struct {
	SessionID string           `json:"session_id"`
	State     OnboardingState  `json:"state"`
	Challenge *DIDAuthChallenge `json:"challenge,omitempty"`
}

// DIDAuthChallenge is the challenge issued during onboarding for proof of key control.
type DIDAuthChallenge struct {
	Type      string    `json:"type"`
	Challenge string    `json:"challenge"`
	Domain    string    `json:"domain"`
	Nonce     string    `json:"nonce"`
	IssuedAt  time.Time `json:"issuedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// DIDAuthResponse is the signed response to a DID-Auth challenge.
type DIDAuthResponse struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	DID       string `json:"did"`
	Proof     Proof  `json:"proof"`
}

// NegotiateRequest is the body of POST /aicp/v1/onboard/{id}/negotiate.
type NegotiateRequest struct {
	SessionID          string              `json:"session_id"`
	ChallengeResponse  *DIDAuthResponse    `json:"challenge_response,omitempty"`
	OfferedCapabilities []CapabilityOffer  `json:"offered_capabilities"`
}

// CapabilityOffer describes a capability the agent wants.
type CapabilityOffer struct {
	Name        string `json:"name"`
	Tier        string `json:"tier,omitempty"`
	Description string `json:"description,omitempty"`
}

// NegotiateResponse is returned from POST /aicp/v1/onboard/{id}/negotiate.
type NegotiateResponse struct {
	State              OnboardingState   `json:"state"`
	ApprovedCapabilities []string        `json:"approved_capabilities"`
	Terms              *ServiceTerms     `json:"terms,omitempty"`
}

// ServiceTerms describes the agreed terms between agent and service.
type ServiceTerms struct {
	PricingTier string      `json:"pricing_tier"`
	Rate        string      `json:"rate,omitempty"`
	Currency    string      `json:"currency,omitempty"`
	BillingCycle string    `json:"billing_cycle,omitempty"`
	RateLimit   int         `json:"rate_limit,omitempty"`
}

// AuthorizeRequest is the body of POST /aicp/v1/onboard/{id}/authorize.
type AuthorizeRequest struct {
	SessionID     string `json:"session_id"`
	PaymentToken  string `json:"payment_token"`
}

// AuthorizeResponse is returned from POST /aicp/v1/onboard/{id}/authorize.
type AuthorizeResponse struct {
	State         OnboardingState `json:"state"`
	Authorized    bool            `json:"authorized"`
}

// ProvisionRequest is the body of POST /aicp/v1/onboard/{id}/provision.
type ProvisionRequest struct {
	SessionID string `json:"session_id"`
}

// ProvisionResponse is returned from POST /aicp/v1/onboard/{id}/provision.
type ProvisionResponse struct {
	State       OnboardingState        `json:"state"`
	AccountID   string                 `json:"account_id"`
	Credentials map[string]string      `json:"credentials"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
}

// ─── Commerce Types ─────────────────────────────────────────────────────────

// PaymentTokenClaims are the custom claims in an AICP Payment Token (JWS).
type PaymentTokenClaims struct {
	JTI             string                `json:"jti"`
	Issuer          string                `json:"iss"`
	Subject         string                `json:"sub"`
	Audience        string                `json:"aud"`
	IssuedAt        int64                 `json:"iat"`
	ExpiresAt       int64                 `json:"exp"`
	PaymentDetails  PaymentTokenDetails   `json:"aicp:payment"`
}

// PaymentTokenDetails holds the payment-specific fields of a token.
type PaymentTokenDetails struct {
	Amount          MoneyAmount `json:"amount"`
	MaxAmount       MoneyAmount `json:"maxAmount"`
	Purpose         string      `json:"purpose"`
	ServiceCategory string      `json:"serviceCategory"`
	AdapterID       string      `json:"adapterId"`
	PrincipalDID    string      `json:"principalDid"`
	Nonce           string      `json:"nonce"`
	IdempotencyKey  string      `json:"idempotencyKey"`
}

// PaymentPurpose enumerates valid payment purposes.
type PaymentPurpose string

const (
	PurposeOnboarding    PaymentPurpose = "service-onboarding"
	PurposeSubscription  PaymentPurpose = "subscription-payment"
	PurposeUsage         PaymentPurpose = "usage-payment"
	PurposeOneTime       PaymentPurpose = "one-time-purchase"
)

// PaymentTokenRequest is the body of POST /aicp/v1/payment/token.
type PaymentTokenRequest struct {
	AgentDID        string      `json:"agent_did"`
	ServiceDID      string      `json:"service_did"`
	Amount          MoneyAmount `json:"amount"`
	MaxAmount       *MoneyAmount `json:"max_amount,omitempty"`
	Purpose         string      `json:"purpose"`
	ServiceCategory string      `json:"service_category"`
}

// PaymentTokenResponse is returned from POST /aicp/v1/payment/token.
type PaymentTokenResponse struct {
	Token     string    `json:"token"`
	TokenID   string    `json:"token_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// PaymentExecuteRequest is the body of POST /aicp/v1/payment/execute.
type PaymentExecuteRequest struct {
	Token string `json:"token"`
}

// PaymentExecuteResponse is returned from POST /aicp/v1/payment/execute.
type PaymentExecuteResponse struct {
	PaymentID   string      `json:"payment_id"`
	Status      string      `json:"status"`
	Amount      MoneyAmount `json:"amount"`
	ReceiptID   string      `json:"receipt_id"`
	ReceiptHash string      `json:"receipt_hash"`
	ApprovalID  string      `json:"approval_id,omitempty"`
}

// ─── Approval Types ─────────────────────────────────────────────────────────

// ApprovalResponse is returned from approval-related endpoints.
type ApprovalResponse struct {
	ApprovalID   string    `json:"approval_id"`
	AgentDID     string    `json:"agent_did"`
	PrincipalDID string    `json:"principal_did"`
	Amount       MoneyAmount `json:"amount"`
	Purpose      string    `json:"purpose"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	DecidedAt    *time.Time `json:"decided_at,omitempty"`
	DecidedBy    string    `json:"decided_by,omitempty"`
}

// ApprovalDecisionRequest is the body for approve/deny endpoints.
type ApprovalDecisionRequest struct {
	Reason string `json:"reason,omitempty"`
}

// CredentialRevokeRequest is the body of POST /aicp/v1/credentials/{id}/revoke.
type CredentialRevokeRequest struct {
	Reason string `json:"reason,omitempty"`
}

// PaymentStatusResponse is returned from GET /aicp/v1/payment/status/{id}.
type PaymentStatusResponse struct {
	PaymentID   string      `json:"payment_id"`
	Status      string      `json:"status"`
	Amount      MoneyAmount `json:"amount"`
	PayerDID    string      `json:"payer_did"`
	PayeeDID    string      `json:"payee_did"`
	Adapter     string      `json:"adapter"`
	CreatedAt   time.Time   `json:"created_at"`
	SettledAt   *time.Time  `json:"settled_at,omitempty"`
}

// ─── Verify Types ───────────────────────────────────────────────────────────

// VerifyRequest is the body of POST /aicp/v1/verify.
type VerifyRequest struct {
	DID          string                  `json:"did"`
	Presentation *VerifiablePresentation `json:"presentation,omitempty"`
}

// VerifyResponse is returned from POST /aicp/v1/verify.
type VerifyResponse struct {
	Valid             bool                 `json:"valid"`
	DID               string               `json:"did"`
	Document          *DIDDocument         `json:"did_document,omitempty"`
	DelegationChain   []string             `json:"delegation_chain,omitempty"`
	TrustScore        int                  `json:"trust_score"`
	CredentialResults []CredentialVerification `json:"credential_results,omitempty"`
	Errors            []string             `json:"errors,omitempty"`
}

// CredentialVerification holds the result of verifying a single VC.
type CredentialVerification struct {
	CredentialID string `json:"credential_id"`
	Type         string `json:"type"`
	Valid        bool   `json:"valid"`
	Error        string `json:"error,omitempty"`
}

// ─── Ledger Types ───────────────────────────────────────────────────────────

// Block represents a finalized ledger block.
type Block struct {
	Number        uint64             `json:"number"`
	Hash          string             `json:"hash"`
	PreviousHash  string             `json:"previous_hash"`
	MerkleRoot    string             `json:"merkle_root"`
	EntryCount    int                `json:"entry_count"`
	ValidatorDID  string             `json:"validator_did"`
	ValidatorSig  string             `json:"validator_sig"`
	Signatures    []ValidatorSig     `json:"signatures,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
}

// ValidatorSig is a signature from a single validator.
type ValidatorSig struct {
	ValidatorDID string `json:"validator_did"`
	Signature    string `json:"signature"`
}

// LedgerEntry is an individual entry within a block.
type LedgerEntry struct {
	ID           uint64    `json:"id"`
	BlockNumber  uint64    `json:"block_number"`
	EntryType    string    `json:"entry_type"`
	EntryHash    string    `json:"entry_hash"`
	EntryDataRef string    `json:"entry_data_ref"`
	RecordedAt   time.Time `json:"recorded_at"`
}

// AnchorReceipt is returned when a DID operation is anchored to the ledger.
type AnchorReceipt struct {
	DID          string    `json:"did"`
	BlockNumber  uint64    `json:"block_number"`
	EntryHash    string    `json:"entry_hash"`
	DocumentHash string    `json:"document_hash"`
	AnchoredAt   time.Time `json:"anchored_at"`
}

// SignedPaymentReceipt is a payment receipt stored on the ledger.
type SignedPaymentReceipt struct {
	ReceiptID      string      `json:"receipt_id"`
	TokenJTI       string      `json:"token_jti"`
	PayerDID       string      `json:"payer_did"`
	PayeeDID       string      `json:"payee_did"`
	Amount         MoneyAmount `json:"amount"`
	ReceiptHash    string      `json:"receipt_hash"`
	Status         string      `json:"status"`
	SettledAt      time.Time   `json:"settled_at"`
}

// RevocationList represents a compressed bitstring for credential revocation.
type RevocationList struct {
	ListID        string    `json:"list_id"`
	IssuerDID     string    `json:"issuer_did"`
	EncodedList   string    `json:"encoded_list"`
	TotalCapacity int       `json:"total_capacity"`
	NextFreeIndex int       `json:"next_free_index"`
	VersionID     int       `json:"version_id"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TrustScore holds a computed trust score and its breakdown.
type TrustScore struct {
	DID        string             `json:"did"`
	Score      int                `json:"score"`
	Components TrustScoreBreakdown `json:"components"`
	ComputedAt time.Time          `json:"computed_at"`
}

// TrustScoreBreakdown is the per-component score breakdown.
type TrustScoreBreakdown struct {
	Identity      int `json:"identity"`
	AccountAge    int `json:"account_age"`
	Transactions  int `json:"transactions"`
	Endorsements  int `json:"endorsements"`
	Credentials   int `json:"credentials"`
	DisputePenalty int `json:"dispute_penalty"`
}

// ─── WebSocket Event Types ──────────────────────────────────────────────────

// EventType enumerates WebSocket event types.
type EventType string

const (
	EventCredentialRevoked     EventType = "credential.revoked"
	EventRevocationListUpdated EventType = "revocation_list.updated"
	EventPaymentAuthorized     EventType = "payment.authorized"
	EventPaymentSettled        EventType = "payment.settled"
	EventPaymentFailed         EventType = "payment.failed"
	EventPaymentRefunded       EventType = "payment.refunded"
	EventDIDCreated            EventType = "did.created"
	EventDIDUpdated            EventType = "did.updated"
	EventDIDDeactivated        EventType = "did.deactivated"
	EventKeyRotated            EventType = "key.rotated"
	EventServiceStatusChanged  EventType = "service.status_changed"
	EventServiceANSUpdated     EventType = "service.ans_updated"
	EventTrustScoreUpdated     EventType = "trust_score.updated"
	EventPaymentApprovalRequired EventType = "payment.approval_required"
	EventPaymentApprovalDecided  EventType = "payment.approval_decided"
	EventHeartbeat               EventType = "system.heartbeat"
	EventError                   EventType = "system.error"
)

// Event is the envelope for all WebSocket events.
type Event struct {
	EventID   string          `json:"eventId"`
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Source    string          `json:"source"`
	Subject   string          `json:"subject"`
	Data      json.RawMessage `json:"data"`
	Signature string          `json:"signature"`
}

// ─── Pagination ─────────────────────────────────────────────────────────────

// PaginationOpts holds cursor-based pagination parameters.
type PaginationOpts struct {
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor,omitempty"`
}

// Parse returns the limit and offset from pagination options with sensible defaults.
func (opts PaginationOpts) Parse() (limit, offset int) {
	limit = opts.Limit
	if limit <= 0 {
		limit = DefaultPageLimit
	}
	if opts.Cursor != "" {
		if parsed, err := strconv.Atoi(opts.Cursor); err == nil {
			offset = parsed
		}
	}
	return
}

// PaginatedResult wraps a page of results with cursor info.
type PaginatedResult[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// ─── Validator Types ────────────────────────────────────────────────────────

// Validator represents a consensus validator node.
type Validator struct {
	DID         string   `json:"did"`
	PublicKey   string   `json:"public_key"`
	GRPCAddress string   `json:"grpc_address"`
	Active      bool     `json:"active"`
	JoinedAt    time.Time `json:"joined_at"`
}

// ProposedBlock is a block proposed by the leader before finalization.
type ProposedBlock struct {
	Number       uint64        `json:"number"`
	PreviousHash string        `json:"previous_hash"`
	MerkleRoot   string        `json:"merkle_root"`
	Entries      []LedgerEntry `json:"entries"`
	ProposerDID  string        `json:"proposer_did"`
	ProposerSig  string        `json:"proposer_sig"`
	Timestamp    time.Time     `json:"timestamp"`
}

// FinalizedBlock is a block that has collected enough validator signatures.
type FinalizedBlock struct {
	Block      Block          `json:"block"`
	Signatures []ValidatorSig `json:"signatures"`
}
