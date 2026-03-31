// ─── DID Types ──────────────────────────────────────────────────────────────

export interface DIDDocument {
  "@context": string[];
  id: string;
  controller: string;
  verificationMethod: VerificationMethod[];
  authentication: string[];
  assertionMethod?: string[];
  capabilityDelegation?: string[];
  service?: ServiceEndpoint[];
  "aicp:principal"?: string;
  "aicp:delegationCredential"?: string;
  created: string;
  updated: string;
  versionId: number;
}

export interface VerificationMethod {
  id: string;
  type: string;
  controller: string;
  publicKeyMultibase: string;
}

export interface ServiceEndpoint {
  id: string;
  type: string;
  serviceEndpoint: string;
}

export interface DIDDocumentMetadata {
  created: string;
  updated: string;
  versionId: number;
  deactivated: boolean;
  nextUpdate?: string;
  documentHash: string;
}

export interface DIDResolutionResult {
  didDocument: DIDDocument | null;
  didDocumentMetadata: DIDDocumentMetadata | null;
}

// ─── Verifiable Credential Types ────────────────────────────────────────────

export interface VerifiableCredential {
  "@context": string[];
  id: string;
  type: string[];
  issuer: string;
  issuanceDate: string;
  expirationDate?: string;
  credentialSubject: Record<string, unknown>;
  credentialStatus?: CredentialStatus;
  proof?: Proof;
}

export interface CredentialStatus {
  id: string;
  type: string;
  revocationListIndex: string;
  revocationListCredential: string;
}

export interface Proof {
  type: string;
  created: string;
  verificationMethod: string;
  proofPurpose: string;
  proofValue: string;
  challenge?: string;
  domain?: string;
}

export interface VerifiablePresentation {
  "@context": string[];
  type: string[];
  holder: string;
  verifiableCredential: VerifiableCredential[];
  proof?: Proof;
}

// ─── Agent Credential Subject Types ─────────────────────────────────────────

export interface AgentDelegationSubject {
  id: string;
  type: string;
  principal: string;
  delegatedCapabilities: string[];
  constraints: DelegationConstraints;
}

export interface DelegationConstraints {
  maxTransactionAmount: MoneyAmount;
  dailySpendingLimit: MoneyAmount;
  monthlySpendingLimit: MoneyAmount;
  allowedServiceCategories: string[];
  allowedGeographies?: string[];
  maxConcurrentSubscriptions?: number;
}

export interface MoneyAmount {
  value: string;
  currency: string;
}

export interface SpendingAuthorizationSubject {
  id: string;
  type: string;
  paymentMethods: PaymentMethodRef[];
  limits: SpendingLimits;
  allowedMerchantCategories?: string[];
  requiresApprovalAbove?: MoneyAmount;
}

export interface PaymentMethodRef {
  type: string;
  provider: string;
  adapterId: string;
  capabilities: string[];
}

export interface SpendingLimits {
  perTransaction: MoneyAmount;
  daily: MoneyAmount;
  monthly: MoneyAmount;
}

// ─── ANS (Agent Name System) Types ──────────────────────────────────────────

export interface ANSRecord {
  aicp_version: string;
  service: ANSServiceInfo;
  onboarding: ANSOnboardingInfo;
  authentication: ANSAuthInfo;
  pricing?: ANSPricingInfo;
  discovery_tags?: string[];
  trust?: ANSTrustInfo;
  endpoints?: ANSEndpoints;
}

export interface ANSServiceInfo {
  name: string;
  did: string;
  description?: string;
  category: string[];
  homepage?: string;
  logoUrl?: string;
  tosUrl?: string;
  privacyPolicyUrl?: string;
  supportContact?: string;
  apiDocsUrl?: string;
}

export interface ANSOnboardingInfo {
  flows: string[];
  required_credentials: string[];
  optional_credentials?: string[];
  accepted_payments?: string[];
  minimum_trust_score?: number;
  compliance_requirements?: string[];
}

export interface ANSAuthInfo {
  did_methods: string[];
  auth_protocols: string[];
  session_ttl_seconds?: number;
}

export interface ANSPricingInfo {
  model: string;
  currency?: string;
  tiers?: ANSPricingTier[];
}

export interface ANSPricingTier {
  name: string;
  rate?: string;
  description?: string;
}

export interface ANSTrustInfo {
  verified_by?: string[];
  established_since?: string;
  certifications?: string[];
}

export interface ANSEndpoints {
  onboard?: string;
  api_base?: string;
  websocket?: string;
  health?: string;
}

// ─── Onboarding Types ───────────────────────────────────────────────────────

export type OnboardingPhase =
  | "initiated"
  | "identity_presented"
  | "capabilities_negotiated"
  | "payment_authorized"
  | "provisioned"
  | "failed";

export interface OnboardingState {
  session_id: string;
  phase: OnboardingPhase;
  agent_did: string;
  service_did: string;
  started_at: string;
  updated_at: string;
  expires_at: string;
  failed_reason?: string;
}

export interface OnboardInitRequest {
  agent_did: string;
  presentation: VerifiablePresentation;
  requested_capabilities: string[];
}

export interface OnboardInitResponse {
  session_id: string;
  state: OnboardingState;
  challenge?: DIDAuthChallenge;
}

export interface DIDAuthChallenge {
  type: string;
  challenge: string;
  domain: string;
  nonce: string;
  issuedAt: string;
  expiresAt: string;
}

export interface DIDAuthResponse {
  type: string;
  challenge: string;
  did: string;
  proof: Proof;
}

export interface NegotiateRequest {
  session_id: string;
  challenge_response?: DIDAuthResponse;
  offered_capabilities: CapabilityOffer[];
}

export interface CapabilityOffer {
  name: string;
  tier?: string;
  description?: string;
}

export interface NegotiateResponse {
  state: OnboardingState;
  approved_capabilities: string[];
  terms?: ServiceTerms;
}

export interface ServiceTerms {
  pricing_tier: string;
  rate?: string;
  currency?: string;
  billing_cycle?: string;
  rate_limit?: number;
}

export interface AuthorizeRequest {
  session_id: string;
  payment_token: string;
}

export interface AuthorizeResponse {
  state: OnboardingState;
  authorized: boolean;
}

export interface ProvisionRequest {
  session_id: string;
}

export interface ProvisionResponse {
  state: OnboardingState;
  account_id: string;
  credentials: Record<string, string>;
  expires_at?: string;
}

// ─── Commerce Types ─────────────────────────────────────────────────────────

export interface PaymentTokenClaims {
  jti: string;
  iss: string;
  sub: string;
  aud: string;
  iat: number;
  exp: number;
  "aicp:payment": PaymentTokenDetails;
}

export interface PaymentTokenDetails {
  amount: MoneyAmount;
  maxAmount: MoneyAmount;
  purpose: string;
  serviceCategory: string;
  adapterId: string;
  principalDid: string;
  nonce: string;
  idempotencyKey: string;
}

export type PaymentPurpose =
  | "service-onboarding"
  | "subscription-payment"
  | "usage-payment"
  | "one-time-purchase";

export interface PaymentTokenRequest {
  agent_did: string;
  service_did: string;
  amount: MoneyAmount;
  max_amount?: MoneyAmount;
  purpose: string;
  service_category: string;
}

export interface PaymentTokenResponse {
  token: string;
  token_id: string;
  expires_at: string;
}

export interface PaymentExecuteRequest {
  token: string;
}

export interface PaymentExecuteResponse {
  payment_id: string;
  status: string;
  amount: MoneyAmount;
  receipt_id: string;
  receipt_hash: string;
}

export interface PaymentStatusResponse {
  payment_id: string;
  status: string;
  amount: MoneyAmount;
  payer_did: string;
  payee_did: string;
  adapter: string;
  created_at: string;
  settled_at?: string;
}

// ─── Verify Types ───────────────────────────────────────────────────────────

export interface VerifyRequest {
  did: string;
  presentation?: VerifiablePresentation;
}

export interface VerifyResponse {
  valid: boolean;
  did: string;
  did_document?: DIDDocument;
  delegation_chain?: string[];
  trust_score: number;
  credential_results?: CredentialVerification[];
  errors?: string[];
}

export interface CredentialVerification {
  credential_id: string;
  type: string;
  valid: boolean;
  error?: string;
}

// ─── Ledger Types ───────────────────────────────────────────────────────────

export interface Block {
  number: number;
  hash: string;
  previous_hash: string;
  merkle_root: string;
  entry_count: number;
  validator_did: string;
  validator_sig: string;
  signatures?: ValidatorSig[];
  created_at: string;
}

export interface ValidatorSig {
  validator_did: string;
  signature: string;
}

export interface TrustScore {
  did: string;
  score: number;
  components: TrustScoreBreakdown;
  computed_at: string;
}

export interface TrustScoreBreakdown {
  identity: number;
  account_age: number;
  transactions: number;
  endorsements: number;
  credentials: number;
  dispute_penalty: number;
}

// ─── WebSocket Event Types ──────────────────────────────────────────────────

export type EventType =
  | "credential.revoked"
  | "revocation_list.updated"
  | "payment.authorized"
  | "payment.settled"
  | "payment.failed"
  | "payment.refunded"
  | "did.created"
  | "did.updated"
  | "did.deactivated"
  | "key.rotated"
  | "service.status_changed"
  | "service.ans_updated"
  | "trust_score.updated"
  | "system.heartbeat"
  | "system.error";

export interface AICPEvent {
  eventId: string;
  type: EventType;
  timestamp: string;
  source: string;
  subject: string;
  data: Record<string, unknown>;
  signature: string;
}

// ─── API Response Envelope ──────────────────────────────────────────────────

export interface APIResponse<T = unknown> {
  success: boolean;
  data?: T;
  error?: APIError;
}

export interface APIError {
  code: string;
  message: string;
  detail?: string;
}

// ─── Pagination ─────────────────────────────────────────────────────────────

export interface PaginationOpts {
  limit: number;
  cursor?: string;
}

export interface PaginatedResult<T> {
  items: T[];
  next_cursor?: string;
  has_more: boolean;
}

// ─── SDK Configuration ──────────────────────────────────────────────────────

export interface AICPClientConfig {
  /** Base URL of the AICP server (e.g., "https://aicp.example.com") */
  serverURL: string;
  /** Agent DID */
  agentDID?: string;
  /** Optional auth token for requests */
  authToken?: string;
  /** Request timeout in milliseconds (default: 30000) */
  timeout?: number;
  /** Custom fetch implementation */
  fetch?: typeof globalThis.fetch;
}

export interface AICPAgentConfig extends AICPClientConfig {
  /** Ed25519 private key (32 bytes) */
  privateKey: Uint8Array;
  /** Principal DID (for delegation chain) */
  principalDID?: string;
}

export interface KeyPair {
  publicKey: Uint8Array;
  privateKey: Uint8Array;
}
