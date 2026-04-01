// Package aicp provides public types, error codes, and constants for the
// Agentic Identity and Commerce Protocol.
package aicp

// Protocol version constants.
const (
	ProtocolVersion = "1.0"
	ProtocolName    = "AICP"
	DIDMethod       = "aicp"
)

// HTTP error code strings used across handlers.
const (
	CodeInvalidRequest    = "INVALID_REQUEST"
	CodeInternalError     = "INTERNAL_ERROR"
	CodeUnauthorized      = "UNAUTHORIZED"
	CodeNotFound          = "NOT_FOUND"
	CodeInvalidTransition = "INVALID_TRANSITION"
	CodeRateLimited       = "RATE_LIMITED"
	CodeAuthFailed        = "AUTH_FAILED"
	CodeTokenError        = "TOKEN_ERROR"
)

// Payment status constants.
const (
	StatusRequiresCapture = "requires_capture"
	StatusCaptured        = "captured"
	StatusSettled         = "settled"
	StatusRefunded        = "refunded"
	StatusSucceeded       = "succeeded"
	StatusPending         = "pending"
	StatusCreated         = "CREATED"
	StatusCompleted       = "COMPLETED"
)

// Identity constants.
const (
	ContextDIDCore           = "https://www.w3.org/ns/did/v1"
	ContextAICP              = "https://aicp-protocol.org/ns/v1"
	ContextCredentials       = "https://www.w3.org/2018/credentials/v1"
	VMTypeEd25519            = "Ed25519VerificationKey2020"
	ProofTypeEd25519Sig2020  = "Ed25519Signature2020"
	ProofPurposeAssertion    = "assertionMethod"
	PrimaryKeyFragment       = "#key-1"
	CredentialIDPrefix       = "urn:aicp:credential:"
	RevocationIDPrefix       = "urn:aicp:revocation:"
	ChallengeTypeDIDAuth     = "DID-Auth"
)

// Pagination defaults.
const DefaultPageLimit = 20

// Trust score constants.
const MaxTrustScore = 1000

// Ranking weights.
const (
	TextRelevanceWeight = 0.6
	TrustScoreWeight    = 0.4
	TrustScoreScale     = 1000.0
)

// Default spending limits.
const (
	DefaultDailySpendingLimit   = "10000"
	DefaultMonthlySpendingLimit = "100000"
)

// Build-time variables injected via -ldflags.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)
