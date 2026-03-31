# AICP Commerce Layer Specification

**Version:** 1.0.0
**Status:** Draft
**Last Updated:** 2026-03-30

---

## 1. Overview

The Commerce Layer handles all payment operations in the AICP protocol. It defines the cryptographically signed payment token format, the token lifecycle, the payment adapter interface, spending limit enforcement, and cryptographic receipt generation. Payment tokens are JWS (JSON Web Signature) documents signed with Ed25519, enabling verifiable, non-repudiable transactions between AI agents and services.

---

## 2. Payment Token

### 2.1 Format

Payment tokens are compact JWS (RFC 7515) documents using the EdDSA algorithm (Ed25519). The content type is `aicp-payment-token+jws`.

**JOSE Header:**

```json
{
  "alg": "EdDSA",
  "typ": "aicp-payment-token+jws",
  "kid": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx#key-1"
}
```

**Payload (Claims):**

```json
{
  "jti": "pt_a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "iss": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy",
  "sub": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx",
  "aud": "did:aicp:9XyZaBcDeFgHiJkLmNoPqR",
  "iat": 1743339600,
  "exp": 1743339900,
  "amount": "25.00",
  "maxAmount": "30.00",
  "currency": "USD",
  "purpose": "usage-payment",
  "serviceCategory": "ai-inference",
  "adapterId": "stripe",
  "principalDid": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy",
  "nonce": "f8a2b1c3d4e5",
  "idempotencyKey": "idem_x9y8z7w6v5u4t3s2"
}
```

### 2.2 Claim Definitions

| Claim | Type | Required | Description |
|---|---|---|---|
| `jti` | string | Yes | Unique token ID. Format: `pt_` prefix + UUID v4. Used for replay prevention. |
| `iss` | string | Yes | Principal DID (the entity authorizing the payment). |
| `sub` | string | Yes | Agent DID (the entity presenting the token). |
| `aud` | string | Yes | Service DID (the intended recipient). |
| `iat` | integer | Yes | Issued-at timestamp (Unix epoch seconds). |
| `exp` | integer | Yes | Expiration timestamp. Default TTL: 5 minutes after `iat`. Maximum TTL: 15 minutes. |
| `amount` | string | Yes | Exact payment amount as a decimal string. |
| `maxAmount` | string | No | Maximum amount the service may charge (for variable-price transactions). |
| `currency` | string | Yes | ISO 4217 currency code. |
| `purpose` | string | Yes | Payment purpose enum (see Section 2.3). |
| `serviceCategory` | string | Yes | Service category from the ANS record. |
| `adapterId` | string | Yes | Payment adapter identifier (e.g., `stripe`, `paypal`, `usdc`). |
| `principalDid` | string | Yes | Principal DID for spending limit lookup. |
| `nonce` | string | Yes | Random 12-character hex string for additional uniqueness. |
| `idempotencyKey` | string | Yes | Client-generated key for safe retries. Format: `idem_` prefix + 16 alphanumeric chars. |

### 2.3 Payment Purposes

| Purpose | Description |
|---|---|
| `service-onboarding` | Initial payment during the onboarding flow |
| `subscription-payment` | Recurring subscription charge |
| `usage-payment` | Pay-per-use charge for consumed resources |
| `one-time-purchase` | Single non-recurring purchase |

### 2.4 Token Serialization

The token is serialized in JWS Compact Serialization format:

```
BASE64URL(JOSE Header) . BASE64URL(Payload) . BASE64URL(Ed25519 Signature)
```

Example (truncated):

```
eyJhbGciOiJFZERTQSIsInR5cCI6ImFpY3AtcGF5bWVudC10b2tlbitqd3MiLCJraWQiOiJkaWQ6YWljcDo3SGszZFF3UHZMbU54UjlzVHVWd1h5OjNGZ0hqS2xNblBxUnNUNXZXeCNrZXktMSJ9.eyJqdGkiOiJwdF9hMWIyYzNkNC1lNWY2LTc4OTAtYWJjZC1lZjEyMzQ1Njc4OTAiLC...
```

---

## 3. Token Lifecycle

### 3.1 Lifecycle Stages

```
  +-------+     +----------+     +---------+     +---------+
  | ISSUE |---->| VALIDATE |---->| EXECUTE |---->| RECEIPT |
  +-------+     +----------+     +---------+     +---------+
                     |                |
                     v                v
                 +--------+      +--------+
                 | REJECT |      | FAILED |
                 +--------+      +--------+
```

### 3.2 Issue

The agent constructs the payment token claims, signs them with its Ed25519 private key using the go-jose library, and serializes the result as a compact JWS.

**Signing pseudocode:**

```
signer    = jose.NewSigner(jose.EdDSA, agentPrivateKey)
claims    = { jti, iss, sub, aud, iat, exp, amount, ... }
token     = signer.Sign(json.Marshal(claims))
compact   = token.CompactSerialize()
```

### 3.3 Validate

The service validates the received token:

1. **Deserialize**: Parse the JWS compact serialization.
2. **Header check**: Verify `alg` is `EdDSA` and `typ` is `aicp-payment-token+jws`.
3. **Key resolution**: Resolve the DID in `kid` to obtain the public key.
4. **Signature verification**: Verify the EdDSA signature.
5. **Audience check**: Confirm `aud` matches the service's own DID.
6. **Expiry check**: Confirm `exp` > current time.
7. **Replay check**: Confirm `jti` has not been seen before (see Section 4).
8. **Nonce check**: Confirm `nonce` has not been seen in the recent window.
9. **Idempotency check**: If `idempotencyKey` was seen before, return the previous result.
10. **Spending limit check**: Verify against the agent's AICPSpendingAuthorization VC limits.

### 3.4 Execute

If validation passes, the service executes the payment through the specified adapter (see Section 6).

### 3.5 Receipt

Upon successful execution, the service generates a cryptographic receipt (see Section 7).

---

## 4. Replay Prevention

Replay prevention uses three complementary mechanisms:

### 4.1 JTI Uniqueness (PostgreSQL)

Every `jti` is recorded in a PostgreSQL table:

```sql
CREATE TABLE payment_token_jtis (
    jti         VARCHAR(64) PRIMARY KEY,
    agent_did   VARCHAR(128) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_jtis_expires ON payment_token_jtis (expires_at);
```

Before processing a token, the service attempts to INSERT the `jti`. If the insert fails (duplicate), the token is rejected with `PAYMENT_TOKEN_REPLAY`.

Expired JTIs are purged by a periodic cleanup job (runs every hour, deletes rows where `expires_at < NOW() - INTERVAL '1 hour'`).

### 4.2 Nonce Cache (Redis)

The `nonce` field provides a secondary replay check using Redis:

```
Key:    payment:nonce:<nonce>
Value:  <jti>
TTL:    900 seconds (15 minutes)
```

If the nonce key already exists, the token is rejected.

### 4.3 Idempotency Key Deduplication

The `idempotencyKey` allows safe retries. If a token with a previously seen idempotency key is received:

1. Look up the previous result in Redis: `payment:idempotency:<idempotencyKey>`.
2. If found, return the stored result without re-executing the payment.
3. If not found (TTL expired), treat as a new request.

```
Key:    payment:idempotency:<idempotencyKey>
Value:  JSON-serialized payment result
TTL:    86400 seconds (24 hours)
```

---

## 5. Payment Execution Pipeline

### 5.1 Pipeline Stages

```
Token Validation
       |
       v
Spending Limit Check
       |
       v
Adapter: CreatePaymentIntent
       |
       v
Adapter: CapturePayment
       |
       v
Receipt Generation
       |
       v
Ledger Recording
```

### 5.2 Spending Limit Check

Before executing payment, the service verifies the transaction against the agent's spending limits from the AICPSpendingAuthorization VC:

1. **Per-transaction limit**: `amount <= maxTransactionAmount`.
2. **Daily limit**: `daily_total + amount <= dailyLimit`.
3. **Monthly limit**: `monthly_total + amount <= monthlyLimit`.

Spending totals are tracked in Redis:

```
Key:    spending:<agentDid>:daily:<YYYY-MM-DD>
Value:  Cumulative daily spend (string, decimal)
TTL:    172800 seconds (48 hours)

Key:    spending:<agentDid>:monthly:<YYYY-MM>
Value:  Cumulative monthly spend (string, decimal)
TTL:    2764800 seconds (32 days)
```

If any limit is exceeded, the token is rejected with `SPENDING_LIMIT_EXCEEDED`.

### 5.3 Two-Phase Payment

Payments follow a two-phase commit pattern:

1. **Intent**: The adapter creates a payment intent (hold/authorization) for the specified amount.
2. **Capture**: After the business logic succeeds, the adapter captures the held funds.

If the business logic fails between intent and capture, the intent is voided/cancelled.

---

## 6. Payment Adapters

### 6.1 Adapter Interface

Payment adapters implement the following strategy interface:

```go
type PaymentAdapter interface {
    // AdapterID returns the unique identifier for this adapter (e.g., "stripe").
    AdapterID() string

    // CreatePaymentIntent creates a hold/authorization for the specified amount.
    // Returns a provider-specific intent ID.
    CreatePaymentIntent(ctx context.Context, req PaymentIntentRequest) (*PaymentIntent, error)

    // CapturePayment captures a previously created payment intent.
    CapturePayment(ctx context.Context, intentID string, amount Amount) (*PaymentCapture, error)

    // RefundPayment refunds a previously captured payment, fully or partially.
    RefundPayment(ctx context.Context, captureID string, amount Amount) (*PaymentRefund, error)

    // GetPaymentStatus retrieves the current status of a payment.
    GetPaymentStatus(ctx context.Context, intentID string) (*PaymentStatus, error)

    // ValidateWebhook verifies the authenticity of a webhook callback from the provider.
    ValidateWebhook(ctx context.Context, headers http.Header, body []byte) (*WebhookEvent, error)
}
```

**Supporting types:**

```go
type PaymentIntentRequest struct {
    Amount        Amount
    Currency      string
    Description   string
    Metadata      map[string]string
    AgentDID      string
    PrincipalDID  string
    TokenRef      string  // encrypted payment method reference
}

type Amount struct {
    Value    string  // decimal string, e.g., "25.00"
    Currency string  // ISO 4217
}

type PaymentIntent struct {
    IntentID    string
    Status      string  // "created", "requires_capture", "cancelled"
    Amount      Amount
    ProviderRef string
    CreatedAt   time.Time
}

type PaymentCapture struct {
    CaptureID   string
    IntentID    string
    Amount      Amount
    Status      string  // "captured", "failed"
    ProviderRef string
    CapturedAt  time.Time
}
```

### 6.2 Stripe Adapter

The Stripe adapter uses the Stripe API to process card payments.

- **CreatePaymentIntent**: Maps to `POST /v1/payment_intents` with `capture_method=manual`.
- **CapturePayment**: Maps to `POST /v1/payment_intents/{id}/capture`.
- **RefundPayment**: Maps to `POST /v1/refunds`.
- **GetPaymentStatus**: Maps to `GET /v1/payment_intents/{id}`.
- **ValidateWebhook**: Verifies Stripe webhook signatures using `stripe-signature` header.

### 6.3 PayPal Adapter

The PayPal adapter uses the PayPal Orders API v2.

- **CreatePaymentIntent**: Maps to `POST /v2/checkout/orders` with `intent=AUTHORIZE`.
- **CapturePayment**: Maps to `POST /v2/checkout/orders/{id}/capture`.
- **RefundPayment**: Maps to `POST /v2/payments/captures/{id}/refund`.
- **GetPaymentStatus**: Maps to `GET /v2/checkout/orders/{id}`.
- **ValidateWebhook**: Verifies PayPal webhook using the PayPal Notification SDK.

### 6.4 USDC/Circle Adapter

The USDC adapter uses the Circle Payments API for stablecoin transactions.

- **CreatePaymentIntent**: Maps to `POST /v1/payments` with `autoCapture=false`.
- **CapturePayment**: Maps to `POST /v1/payments/{id}/capture`.
- **RefundPayment**: Maps to `POST /v1/payments/{id}/refund`.
- **GetPaymentStatus**: Maps to `GET /v1/payments/{id}`.
- **ValidateWebhook**: Verifies Circle notification signatures.

---

## 7. Cryptographic Receipts

### 7.1 Receipt Structure

After successful payment capture, the service generates a cryptographic receipt:

```json
{
  "receiptId": "rcpt_m1n2o3p4q5r6",
  "transactionId": "tx_a1b2c3d4e5f6",
  "paymentTokenJti": "pt_a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "agentDid": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx",
  "serviceDid": "did:aicp:9XyZaBcDeFgHiJkLmNoPqR",
  "principalDid": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy",
  "amount": "25.00",
  "currency": "USD",
  "purpose": "usage-payment",
  "adapterId": "stripe",
  "providerRef": "pi_3abc123def456",
  "timestamp": "2026-03-30T12:05:00Z",
  "transactionHash": "sha256:b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5",
  "proof": {
    "type": "Ed25519Signature2020",
    "created": "2026-03-30T12:05:00Z",
    "verificationMethod": "did:aicp:9XyZaBcDeFgHiJkLmNoPqR#key-1",
    "proofPurpose": "assertionMethod",
    "proofValue": "z3Kp8Q2rS..."
  }
}
```

### 7.2 Transaction Hash

The `transactionHash` is computed as:

```
fields = receiptId + transactionId + paymentTokenJti + agentDid
       + serviceDid + principalDid + amount + currency + purpose
       + adapterId + providerRef + timestamp
hash   = SHA-256(fields)
transactionHash = "sha256:" + hex.Encode(hash)
```

Fields are concatenated in the order listed, with no separator. This deterministic ordering ensures any party can independently verify the hash.

### 7.3 Ledger Recording

The receipt hash (and optionally the full receipt) is recorded on the consortium ledger, providing an immutable audit trail. The ledger entry includes:

```json
{
  "type": "payment_receipt",
  "transactionHash": "sha256:b4c5d6e7f8a9...",
  "participants": [
    "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx",
    "did:aicp:9XyZaBcDeFgHiJkLmNoPqR"
  ],
  "timestamp": "2026-03-30T12:05:00Z",
  "blockHeight": 1847293
}
```

---

## 8. Spending Limits

### 8.1 Limit Sources

Spending limits are defined in the `AICPSpendingAuthorization` Verifiable Credential issued by the principal:

| Limit Type | Field | Description |
|---|---|---|
| Per-transaction | `maxTransactionAmount` | Maximum amount for a single payment |
| Daily | `dailyLimit` | Maximum cumulative spend per calendar day (UTC) |
| Monthly | `monthlyLimit` | Maximum cumulative spend per calendar month (UTC) |

### 8.2 Enforcement

```
function checkSpendingLimits(agentDid, amount, currency, vc):
    // Per-transaction check
    if amount > vc.limits.maxTransactionAmount:
        return SPENDING_LIMIT_EXCEEDED

    // Daily check
    dailyKey   = "spending:" + agentDid + ":daily:" + today()
    dailyTotal = redis.GET(dailyKey) or "0"
    if decimal(dailyTotal) + decimal(amount) > vc.limits.dailyLimit:
        return SPENDING_LIMIT_EXCEEDED

    // Monthly check
    monthlyKey   = "spending:" + agentDid + ":monthly:" + thisMonth()
    monthlyTotal = redis.GET(monthlyKey) or "0"
    if decimal(monthlyTotal) + decimal(amount) > vc.limits.monthlyLimit:
        return SPENDING_LIMIT_EXCEEDED

    return OK
```

### 8.3 Limit Tracking

After a successful payment capture, the spending counters are atomically incremented:

```
INCRBYFLOAT spending:<agentDid>:daily:<YYYY-MM-DD> <amount>
INCRBYFLOAT spending:<agentDid>:monthly:<YYYY-MM> <amount>
```

### 8.4 Limit Queries

Agents can query their current spending status:

```http
GET /aicp/v1/spending/did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx
Authorization: DID did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx z3Kp8Q2rS...
```

**Response:**

```json
{
  "data": {
    "agentDid": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx",
    "limits": {
      "perTransaction": { "max": "100.00", "currency": "USD" },
      "daily": { "max": "500.00", "used": "127.50", "remaining": "372.50", "currency": "USD" },
      "monthly": { "max": "5000.00", "used": "1842.00", "remaining": "3158.00", "currency": "USD" }
    },
    "resetTimes": {
      "daily": "2026-03-31T00:00:00Z",
      "monthly": "2026-04-01T00:00:00Z"
    }
  }
}
```

---

## 9. Security Considerations

- **Token TTL**: Payment tokens MUST expire within 15 minutes of issuance. The default TTL of 5 minutes is strongly recommended. Services MUST reject tokens with `exp - iat > 900`.
- **Amount precision**: Amounts are represented as decimal strings to avoid floating-point rounding errors. Implementations MUST use arbitrary-precision decimal arithmetic.
- **Key binding**: The `kid` header in the JWS must match a key in the agent's DID Document. The `sub` claim must match the DID in `kid`.
- **Audience binding**: The `aud` claim prevents a token issued for one service from being presented to another.
- **Idempotency window**: The 24-hour idempotency window balances retry safety with storage costs. After the window, duplicate idempotency keys are treated as new requests.
- **Adapter credentials**: Payment adapter credentials (Stripe API keys, PayPal client secrets) MUST be stored in a secrets manager, never in configuration files or environment variables accessible to application code.
- **Receipt immutability**: Once a receipt hash is recorded on the consortium ledger, it cannot be altered. Disputes are resolved by comparing the receipt hash against the independently computed hash from the receipt fields.
