"""Protocol types matching the Go server's pkg/aicp/types.go exactly."""

from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field


# ─── DID Types ──────────────────────────────────────────────────────────────


class VerificationMethod(BaseModel):
    id: str
    type: str
    controller: str
    public_key_multibase: str = Field(alias="publicKeyMultibase")

    model_config = {"populate_by_name": True}


class ServiceEndpoint(BaseModel):
    id: str
    type: str
    service_endpoint: str = Field(alias="serviceEndpoint")

    model_config = {"populate_by_name": True}


class DIDDocument(BaseModel):
    context: list[str] = Field(alias="@context")
    id: str
    controller: str
    verification_method: list[VerificationMethod] = Field(alias="verificationMethod")
    authentication: list[str]
    assertion_method: list[str] | None = Field(None, alias="assertionMethod")
    capability_delegation: list[str] | None = Field(None, alias="capabilityDelegation")
    service: list[ServiceEndpoint] | None = None
    aicp_principal: str | None = Field(None, alias="aicp:principal")
    aicp_delegation_credential: str | None = Field(None, alias="aicp:delegationCredential")
    created: str
    updated: str
    version_id: int = Field(alias="versionId")

    model_config = {"populate_by_name": True}


class DIDDocumentMetadata(BaseModel):
    created: str
    updated: str
    version_id: int = Field(alias="versionId")
    deactivated: bool
    next_update: str | None = Field(None, alias="nextUpdate")
    document_hash: str = Field(alias="documentHash")

    model_config = {"populate_by_name": True}


class DIDResolutionResult(BaseModel):
    did_document: DIDDocument | None = Field(None, alias="didDocument")
    did_document_metadata: DIDDocumentMetadata | None = Field(
        None, alias="didDocumentMetadata"
    )

    model_config = {"populate_by_name": True}


# ─── Verifiable Credential Types ────────────────────────────────────────────


class CredentialStatus(BaseModel):
    id: str
    type: str
    revocation_list_index: str = Field(alias="revocationListIndex")
    revocation_list_credential: str = Field(alias="revocationListCredential")

    model_config = {"populate_by_name": True}


class Proof(BaseModel):
    type: str
    created: str
    verification_method: str = Field(alias="verificationMethod")
    proof_purpose: str = Field(alias="proofPurpose")
    proof_value: str = Field(alias="proofValue")
    challenge: str | None = None
    domain: str | None = None

    model_config = {"populate_by_name": True}


class VerifiableCredential(BaseModel):
    context: list[str] = Field(alias="@context")
    id: str
    type: list[str]
    issuer: str
    issuance_date: str = Field(alias="issuanceDate")
    expiration_date: str | None = Field(None, alias="expirationDate")
    credential_subject: dict[str, Any] = Field(alias="credentialSubject")
    credential_status: CredentialStatus | None = Field(None, alias="credentialStatus")
    proof: Proof | None = None

    model_config = {"populate_by_name": True}


class VerifiablePresentation(BaseModel):
    context: list[str] = Field(alias="@context")
    type: list[str]
    holder: str
    verifiable_credential: list[VerifiableCredential] = Field(
        alias="verifiableCredential"
    )
    proof: Proof | None = None

    model_config = {"populate_by_name": True}


# ─── Money & Credential Subject Types ───────────────────────────────────────


class MoneyAmount(BaseModel):
    value: str
    currency: str


class DelegationConstraints(BaseModel):
    max_transaction_amount: MoneyAmount = Field(alias="maxTransactionAmount")
    daily_spending_limit: MoneyAmount = Field(alias="dailySpendingLimit")
    monthly_spending_limit: MoneyAmount = Field(alias="monthlySpendingLimit")
    allowed_service_categories: list[str] = Field(alias="allowedServiceCategories")
    allowed_geographies: list[str] | None = Field(None, alias="allowedGeographies")
    max_concurrent_subscriptions: int | None = Field(
        None, alias="maxConcurrentSubscriptions"
    )

    model_config = {"populate_by_name": True}


class SpendingLimits(BaseModel):
    per_transaction: MoneyAmount = Field(alias="perTransaction")
    daily: MoneyAmount
    monthly: MoneyAmount

    model_config = {"populate_by_name": True}


# ─── ANS (Agent Name System) Types ──────────────────────────────────────────


class ANSServiceInfo(BaseModel):
    name: str
    did: str
    description: str | None = None
    category: list[str]
    homepage: str | None = None
    logo_url: str | None = Field(None, alias="logoUrl")
    tos_url: str | None = Field(None, alias="tosUrl")
    privacy_policy_url: str | None = Field(None, alias="privacyPolicyUrl")
    support_contact: str | None = Field(None, alias="supportContact")
    api_docs_url: str | None = Field(None, alias="apiDocsUrl")

    model_config = {"populate_by_name": True}


class ANSOnboardingInfo(BaseModel):
    flows: list[str]
    required_credentials: list[str]
    optional_credentials: list[str] | None = None
    accepted_payments: list[str] | None = None
    minimum_trust_score: int | None = None
    compliance_requirements: list[str] | None = None


class ANSAuthInfo(BaseModel):
    did_methods: list[str]
    auth_protocols: list[str]
    session_ttl_seconds: int | None = None


class ANSPricingTier(BaseModel):
    name: str
    rate: str | None = None
    description: str | None = None


class ANSPricingInfo(BaseModel):
    model: str
    currency: str | None = None
    tiers: list[ANSPricingTier] | None = None


class ANSTrustInfo(BaseModel):
    verified_by: list[str] | None = None
    established_since: str | None = None
    certifications: list[str] | None = None


class ANSEndpoints(BaseModel):
    onboard: str | None = None
    api_base: str | None = None
    websocket: str | None = None
    health: str | None = None


class ANSRecord(BaseModel):
    aicp_version: str
    service: ANSServiceInfo
    onboarding: ANSOnboardingInfo
    authentication: ANSAuthInfo
    pricing: ANSPricingInfo | None = None
    discovery_tags: list[str] | None = None
    trust: ANSTrustInfo | None = None
    endpoints: ANSEndpoints | None = None


# ─── Onboarding Types ───────────────────────────────────────────────────────


class OnboardingState(BaseModel):
    session_id: str
    phase: str
    agent_did: str
    service_did: str
    started_at: str
    updated_at: str
    expires_at: str
    failed_reason: str | None = None


class DIDAuthChallenge(BaseModel):
    type: str
    challenge: str
    domain: str
    nonce: str
    issued_at: str = Field(alias="issuedAt")
    expires_at: str = Field(alias="expiresAt")

    model_config = {"populate_by_name": True}


class DIDAuthResponse(BaseModel):
    type: str
    challenge: str
    did: str
    proof: Proof


class OnboardInitResponse(BaseModel):
    session_id: str
    state: OnboardingState
    challenge: DIDAuthChallenge | None = None


class NegotiateResponse(BaseModel):
    state: OnboardingState
    approved_capabilities: list[str]
    terms: dict[str, Any] | None = None


class AuthorizeResponse(BaseModel):
    state: OnboardingState
    authorized: bool


class ProvisionResponse(BaseModel):
    state: OnboardingState
    account_id: str
    credentials: dict[str, str]
    expires_at: str | None = None


# ─── Commerce Types ─────────────────────────────────────────────────────────


class PaymentTokenResponse(BaseModel):
    token: str
    token_id: str
    expires_at: str


class PaymentExecuteResponse(BaseModel):
    payment_id: str
    status: str
    amount: MoneyAmount
    receipt_id: str
    receipt_hash: str


class PaymentStatusResponse(BaseModel):
    payment_id: str
    status: str
    amount: MoneyAmount
    payer_did: str
    payee_did: str
    adapter: str
    created_at: str
    settled_at: str | None = None


# ─── Verify Types ───────────────────────────────────────────────────────────


class CredentialVerification(BaseModel):
    credential_id: str
    type: str
    valid: bool
    error: str | None = None


class VerifyResponse(BaseModel):
    valid: bool
    did: str
    did_document: DIDDocument | None = None
    delegation_chain: list[str] | None = None
    trust_score: int
    credential_results: list[CredentialVerification] | None = None
    errors: list[str] | None = None


# ─── API Response ────────────────────────────────────────────────────────────


class APIError(BaseModel):
    code: str
    message: str
    detail: str | None = None


class APIResponse(BaseModel):
    success: bool
    data: Any = None
    error: APIError | None = None


# ─── Events ─────────────────────────────────────────────────────────────────


class AICPEvent(BaseModel):
    event_id: str = Field(alias="eventId")
    type: str
    timestamp: str
    source: str
    subject: str
    data: dict[str, Any]
    signature: str

    model_config = {"populate_by_name": True}


# ─── Trust Score ─────────────────────────────────────────────────────────────


class TrustScoreBreakdown(BaseModel):
    identity: int
    account_age: int
    transactions: int
    endorsements: int
    credentials: int
    dispute_penalty: int


class TrustScore(BaseModel):
    did: str
    score: int
    components: TrustScoreBreakdown
    computed_at: str
