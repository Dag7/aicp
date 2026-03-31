// ─── Main Client ────────────────────────────────────────────────────────────
export { AICPAgent, AICPClient } from "./client.js";

// ─── Managers ───────────────────────────────────────────────────────────────
export { IdentityManager, Identity } from "./identity.js";
export { DiscoveryManager } from "./discovery.js";
export { OnboardingManager } from "./onboarding.js";
export { CommerceManager } from "./commerce.js";

// ─── Wallet ─────────────────────────────────────────────────────────────────
export {
  CredentialWallet,
  MemoryWalletStorage,
  FileWalletStorage,
} from "./wallet.js";
export type { WalletStorage } from "./wallet.js";

// ─── WebSocket ──────────────────────────────────────────────────────────────
export { EventClient } from "./websocket.js";

// ─── Service Provider ───────────────────────────────────────────────────────
export { AICPServiceAdapter } from "./provider.js";
export type {
  ServiceProviderConfig,
  AICPRequestContext,
} from "./provider.js";

// ─── Transport ──────────────────────────────────────────────────────────────
export { Transport, AICPError } from "./transport.js";

// ─── Crypto ─────────────────────────────────────────────────────────────────
export {
  generateKeyPair,
  getPublicKey,
  sign,
  verify,
  signJSON,
  verifyJSON,
  principalDID,
  agentDID,
  parseDID,
  isPrincipalDID,
  isAgentDID,
  didIdentifier,
  multibaseEncode,
  multibaseDecode,
  createProof,
  verifyProof,
  hash,
  hashHex,
  base58Encode,
  base58Decode,
  bytesToHex,
  hexToBytes,
  randomBytes,
} from "./crypto.js";

// ─── Types ──────────────────────────────────────────────────────────────────
export type {
  // Config
  AICPClientConfig,
  AICPAgentConfig,
  KeyPair,

  // DID
  DIDDocument,
  VerificationMethod,
  ServiceEndpoint,
  DIDDocumentMetadata,
  DIDResolutionResult,

  // Credentials
  VerifiableCredential,
  CredentialStatus,
  Proof,
  VerifiablePresentation,
  AgentDelegationSubject,
  DelegationConstraints,
  MoneyAmount,
  SpendingAuthorizationSubject,
  PaymentMethodRef,
  SpendingLimits,

  // ANS
  ANSRecord,
  ANSServiceInfo,
  ANSOnboardingInfo,
  ANSAuthInfo,
  ANSPricingInfo,
  ANSPricingTier,
  ANSTrustInfo,
  ANSEndpoints,

  // Onboarding
  OnboardingPhase,
  OnboardingState,
  OnboardInitRequest,
  OnboardInitResponse,
  DIDAuthChallenge,
  DIDAuthResponse,
  NegotiateRequest,
  NegotiateResponse,
  CapabilityOffer,
  ServiceTerms,
  AuthorizeRequest,
  AuthorizeResponse,
  ProvisionRequest,
  ProvisionResponse,

  // Commerce
  PaymentTokenClaims,
  PaymentTokenDetails,
  PaymentPurpose,
  PaymentTokenRequest,
  PaymentTokenResponse,
  PaymentExecuteRequest,
  PaymentExecuteResponse,
  PaymentStatusResponse,

  // Verify
  VerifyRequest,
  VerifyResponse,
  CredentialVerification,

  // Ledger
  Block,
  ValidatorSig,
  TrustScore,
  TrustScoreBreakdown,

  // Events
  EventType,
  AICPEvent,

  // API
  APIResponse,
  APIError,
  PaginationOpts,
  PaginatedResult,
} from "./types.js";
