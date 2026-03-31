# AICP Onboarding Layer Specification

**Version:** 1.0.0
**Status:** Draft
**Last Updated:** 2026-03-30

---

## 1. Overview

The Onboarding Layer defines a structured, 4-phase state machine that governs how an AI agent establishes a trust relationship and service account with an AICP-compatible service. Every onboarding session progresses through identity presentation, capability negotiation, payment authorization, and provisioning --- or transitions to a terminal failure state.

---

## 2. State Machine

### 2.1 States

| State | Description |
|---|---|
| `initiated` | Session created, awaiting agent identity |
| `identity_presented` | Agent has authenticated via DID-Auth |
| `capabilities_negotiated` | Agent and service have agreed on capabilities and terms |
| `payment_authorized` | Payment method has been validated and pre-authorized |
| `provisioned` | Account is active and ready for use |
| `failed` | Terminal failure state |

### 2.2 State Transition Diagram

```
                    +---------+
                    | FAILED  |
                    +----^----+
                         |
          (any phase can fail)
                         |
  +----------+    +-----------+    +--------------+    +------------+    +-------------+
  | INITIATED|--->| IDENTITY  |--->| CAPABILITIES |--->| PAYMENT    |--->| PROVISIONED |
  |          |    | PRESENTED |    | NEGOTIATED   |    | AUTHORIZED |    |             |
  +----------+    +-----------+    +--------------+    +------------+    +-------------+
       Phase 1         Phase 2          Phase 3             Phase 4
```

### 2.3 State Transitions

| From | To | Trigger | Guard |
|---|---|---|---|
| `initiated` | `identity_presented` | Agent submits VerifiablePresentation | DID-Auth challenge-response valid |
| `identity_presented` | `capabilities_negotiated` | Service offers capabilities, agent accepts | Trust score meets minimum |
| `capabilities_negotiated` | `payment_authorized` | Agent submits payment token | Payment pre-authorization succeeds |
| `payment_authorized` | `provisioned` | Service provisions account | All credentials issued |
| Any state | `failed` | Validation failure, timeout, or rejection | Error condition detected |

---

## 3. Phase 1: Initiation and Identity Presentation

### 3.1 Session Initiation

The agent initiates onboarding by requesting a DID-Auth challenge from the service.

**Request:**

```http
POST /aicp/v1/onboard/initiate
Content-Type: application/json

{
  "agentDid": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx"
}
```

**Response:**

```json
{
  "data": {
    "sessionId": "sess_a1b2c3d4e5f6",
    "phase": "initiated",
    "challenge": {
      "nonce": "9a4f2c8e-3b7d-4a1f-b5e2-1c8d9f0a3b6e",
      "domain": "cloudinference.example.com",
      "expiresAt": "2026-03-30T12:05:00Z"
    }
  }
}
```

### 3.2 DID-Auth Challenge-Response

The service issues a challenge containing:

| Field | Type | Description |
|---|---|---|
| `nonce` | string | UUID v4, single-use, expires in 5 minutes |
| `domain` | string | The service's domain (prevents cross-service replay) |
| `expiresAt` | string | ISO 8601 expiry timestamp |

The agent constructs a VerifiablePresentation containing:
1. The `AICPAgentDelegation` credential.
2. Optionally, the `AICPSpendingAuthorization` credential.
3. A proof over the presentation, signed with the agent's key, including the challenge nonce and domain.

**Request:**

```http
POST /aicp/v1/onboard/sessions/{sessionId}/identity
Content-Type: application/json

{
  "verifiablePresentation": {
    "@context": [
      "https://www.w3.org/2018/credentials/v1",
      "https://aicp.dev/ns/v1"
    ],
    "type": ["VerifiablePresentation"],
    "holder": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx",
    "verifiableCredential": [
      { "...AICPAgentDelegation credential..." },
      { "...AICPSpendingAuthorization credential..." }
    ],
    "proof": {
      "type": "Ed25519Signature2020",
      "created": "2026-03-30T12:01:00Z",
      "verificationMethod": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx#key-1",
      "proofPurpose": "authentication",
      "proofValue": "z4Kj7V9eQ3...",
      "challenge": "9a4f2c8e-3b7d-4a1f-b5e2-1c8d9f0a3b6e",
      "domain": "cloudinference.example.com"
    }
  }
}
```

### 3.3 Identity Verification

The service validates the presentation:

1. **Proof verification**: Verify the Ed25519 signature against the agent's public key.
2. **Challenge match**: Confirm the `challenge` in the proof matches the issued nonce.
3. **Domain match**: Confirm the `domain` in the proof matches the service's domain.
4. **Expiry check**: Confirm the challenge has not expired.
5. **DID resolution**: Resolve the agent's DID and confirm it is active.
6. **Delegation validation**: Verify the `AICPAgentDelegation` credential:
   - Signature is valid (issued by the principal).
   - Credential is not expired.
   - Credential is not revoked (check RevocationList2020).
   - The principal DID (issuer) is active.
7. **Controller check**: Confirm the agent DID's `controller` matches the delegation issuer.

If all checks pass, the session transitions to `identity_presented`.

**Response:**

```json
{
  "data": {
    "sessionId": "sess_a1b2c3d4e5f6",
    "phase": "identity_presented",
    "trustScore": 620,
    "nextStep": "negotiate_capabilities"
  }
}
```

---

## 4. Phase 2: Capability Negotiation

### 4.1 Trust Score Evaluation

Upon successful identity presentation, the service computes the agent's trust score.

### 4.2 Trust Score Calculation

The trust score is a composite value on a scale of 0 to 1000, computed as follows:

| Component | Weight | Max Points | Calculation |
|---|---|---|---|
| Identity verification | 25% | 250 | DID age, controller verification, key management practices |
| Account age | 15% | 150 | Linear scale: 0 at creation, 150 at 365+ days |
| Successful transactions | 20% | 200 | Logarithmic scale: `min(200, 40 * ln(1 + tx_count))` |
| Service endorsements | 15% | 150 | Count and quality of AICPServiceAttestation VCs |
| Credential completeness | 10% | 100 | Ratio of presented credentials to requested credentials |
| Dispute history | 15% | -150 | Penalty: `-15 * dispute_count` (capped at -150) |

**Formula:**

```
trust_score = identity_score + age_score + transaction_score
            + endorsement_score + completeness_score + dispute_penalty

trust_score = max(0, min(1000, trust_score))
```

### 4.3 Trust Score Thresholds

Services define minimum trust score thresholds in their ANS record (`onboarding.minimum_trust_score`). If the agent's score falls below this threshold, the service MAY:

1. Reject the onboarding entirely (transition to `failed`).
2. Offer a restricted capability set.
3. Require additional credentials or verification.

### 4.4 Capability Offer

The service evaluates the agent's delegated capabilities (from the AICPAgentDelegation VC) against its own service offerings and the trust score, then presents an offer:

**Request:**

```http
POST /aicp/v1/onboard/sessions/{sessionId}/capabilities
Content-Type: application/json

{
  "requestedCapabilities": [
    "ai-inference.standard",
    "ai-inference.premium"
  ]
}
```

**Response:**

```json
{
  "data": {
    "sessionId": "sess_a1b2c3d4e5f6",
    "phase": "capabilities_negotiated",
    "offeredCapabilities": [
      {
        "capability": "ai-inference.standard",
        "status": "granted",
        "rateLimit": 1000,
        "pricing": {
          "unit": "request",
          "unitPrice": "0.002",
          "currency": "USD"
        }
      },
      {
        "capability": "ai-inference.premium",
        "status": "restricted",
        "reason": "Trust score below 700 for premium tier",
        "rateLimit": 100,
        "pricing": {
          "unit": "request",
          "unitPrice": "0.01",
          "currency": "USD"
        }
      }
    ],
    "termsHash": "sha256:a3f2b8c1d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1",
    "nextStep": "authorize_payment"
  }
}
```

The `termsHash` is a SHA-256 hash of the full terms document. The agent includes this hash in the payment authorization step to prove it agreed to the terms.

---

## 5. Phase 3: Payment Authorization

### 5.1 Payment Token Submission

The agent submits a payment token (see Commerce Layer, 04-commerce-layer.md) to authorize payment for the negotiated capabilities.

**Request:**

```http
POST /aicp/v1/onboard/sessions/{sessionId}/payment
Content-Type: application/json

{
  "paymentToken": "eyJhbGciOiJFZERTQSIsInR5cCI6ImFpY3AtcGF5bWVudC10b2tlbitqd3MifQ...",
  "termsHash": "sha256:a3f2b8c1d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1"
}
```

### 5.2 Payment Validation

The service validates the payment token:

1. Verify JWS signature (EdDSA with agent's key).
2. Confirm `sub` claim matches the agent DID in the session.
3. Confirm `aud` claim matches the service DID.
4. Confirm `exp` has not passed.
5. Confirm `jti` has not been seen before (replay prevention).
6. Verify spending limits from AICPSpendingAuthorization VC.
7. Execute payment pre-authorization via the specified adapter.

**Response (success):**

```json
{
  "data": {
    "sessionId": "sess_a1b2c3d4e5f6",
    "phase": "payment_authorized",
    "paymentStatus": "pre_authorized",
    "nextStep": "provision"
  }
}
```

---

## 6. Phase 4: Provisioning

### 6.1 Account Provisioning

Upon successful payment authorization, the service provisions the agent's account:

1. Creates an internal account record linked to the agent DID.
2. Issues an API key or session token for ongoing access.
3. Applies the negotiated capability grants and rate limits.
4. Captures the pre-authorized payment (if applicable).
5. Issues a cryptographic receipt for the transaction.

**Response:**

```json
{
  "data": {
    "sessionId": "sess_a1b2c3d4e5f6",
    "phase": "provisioned",
    "account": {
      "accountId": "acct_x9y8z7w6v5u4",
      "agentDid": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx",
      "capabilities": [
        {
          "capability": "ai-inference.standard",
          "rateLimit": 1000
        },
        {
          "capability": "ai-inference.premium",
          "rateLimit": 100
        }
      ],
      "apiKey": "aicp_live_k3y_a1b2c3d4e5f6g7h8i9j0",
      "expiresAt": "2027-03-30T12:05:00Z"
    },
    "receipt": {
      "receiptId": "rcpt_m1n2o3p4q5r6",
      "transactionHash": "sha256:b4c5d6e7f8a9...",
      "amount": "0.00",
      "currency": "USD",
      "timestamp": "2026-03-30T12:05:00Z"
    }
  }
}
```

---

## 7. Session Management

### 7.1 Session Storage

Onboarding sessions are stored in Redis with the following structure:

```
Key:    onboard:session:<sessionId>
TTL:    1800 seconds (30 minutes, configurable)
```

**Session Data:**

```json
{
  "sessionId": "sess_a1b2c3d4e5f6",
  "agentDid": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx",
  "principalDid": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy",
  "phase": "identity_presented",
  "trustScore": 620,
  "challengeNonce": "9a4f2c8e-3b7d-4a1f-b5e2-1c8d9f0a3b6e",
  "offeredCapabilities": [],
  "paymentTokenJti": null,
  "createdAt": "2026-03-30T12:00:00Z",
  "updatedAt": "2026-03-30T12:01:00Z",
  "expiresAt": "2026-03-30T12:30:00Z"
}
```

### 7.2 Session Expiry

If the session TTL expires before provisioning completes, the session transitions to `failed` with reason `SESSION_EXPIRED`. The agent must start a new onboarding session from the beginning.

### 7.3 Session ID Format

Session IDs are prefixed with `sess_` followed by 12 random alphanumeric characters, generated using a cryptographically secure random number generator.

---

## 8. Anti-Spam and Rate Limiting

### 8.1 Rate Limiting

Onboarding attempts are rate-limited per DID:

| Scope | Limit | Window |
|---|---|---|
| Session initiation per agent DID | 10 | 1 hour |
| Session initiation per principal DID | 50 | 1 hour |
| Failed attempts per agent DID | 5 | 15 minutes |
| Global session initiation | 1000 | 1 minute |

Rate limit state is tracked in Redis using sliding window counters:

```
Key:    ratelimit:onboard:<agentDid>:<window>
TTL:    Matches the window duration
```

### 8.2 Anomaly Detection

The onboarding layer implements two anomaly detection mechanisms:

#### Burst Detection

If an agent DID initiates more than 3 sessions within 60 seconds, subsequent requests are delayed with exponential backoff:

```
delay = min(30s, 2^(burst_count - 3) seconds)
```

#### Geographic Anomaly

If an agent DID presents from a geographic region significantly different from its historical pattern (based on IP geolocation), the service MAY:

1. Flag the session for enhanced verification.
2. Require a higher trust score threshold.
3. Reject the session if the geographic shift exceeds policy limits.

---

## 9. State Transition Details

### 9.1 Transition Guards

Each state transition has guards that must be satisfied:

#### `initiated` -> `identity_presented`

- Challenge nonce is valid and not expired.
- VerifiablePresentation proof is cryptographically valid.
- Agent DID is active (not deactivated).
- AICPAgentDelegation credential is valid, not expired, not revoked.
- Principal DID (delegation issuer) is active.

#### `identity_presented` -> `capabilities_negotiated`

- Trust score has been computed.
- Trust score meets the service's minimum threshold (or restricted offer is acceptable).
- At least one capability was granted or restricted (not all rejected).

#### `capabilities_negotiated` -> `payment_authorized`

- Payment token is cryptographically valid.
- Payment token claims match the session context.
- Payment token has not been replayed.
- Spending limits are not exceeded.
- Payment adapter pre-authorization succeeds.
- `termsHash` in the request matches the hash issued during negotiation.

#### `payment_authorized` -> `provisioned`

- Payment capture succeeds (or is not required for $0 onboarding).
- Account record is created in the service's database.
- API credentials are generated.

### 9.2 Failure Transitions

Any state can transition to `failed`. Common failure reasons:

| Reason Code | From State(s) | Description |
|---|---|---|
| `CHALLENGE_EXPIRED` | initiated | DID-Auth challenge TTL exceeded |
| `INVALID_PROOF` | initiated | VerifiablePresentation proof verification failed |
| `DID_DEACTIVATED` | initiated | Agent or principal DID is deactivated |
| `CREDENTIAL_REVOKED` | initiated | AICPAgentDelegation has been revoked |
| `CREDENTIAL_EXPIRED` | initiated | AICPAgentDelegation has expired |
| `TRUST_SCORE_TOO_LOW` | identity_presented | Trust score below minimum, no restricted offer possible |
| `PAYMENT_TOKEN_INVALID` | capabilities_negotiated | Payment token validation failed |
| `SPENDING_LIMIT_EXCEEDED` | capabilities_negotiated | Transaction exceeds agent's spending authority |
| `PAYMENT_FAILED` | capabilities_negotiated | Payment adapter returned an error |
| `PROVISIONING_ERROR` | payment_authorized | Internal error during account provisioning |
| `SESSION_EXPIRED` | Any | Session TTL exceeded |

### 9.3 Event Emission

Each state transition emits an event via the WebSocket layer:

| Event Type | Payload |
|---|---|
| `onboard.session.initiated` | `{ sessionId, agentDid }` |
| `onboard.session.identity_presented` | `{ sessionId, agentDid, trustScore }` |
| `onboard.session.capabilities_negotiated` | `{ sessionId, agentDid, capabilityCount }` |
| `onboard.session.payment_authorized` | `{ sessionId, agentDid, amount }` |
| `onboard.session.provisioned` | `{ sessionId, agentDid, accountId }` |
| `onboard.session.failed` | `{ sessionId, agentDid, reason, phase }` |

---

## 10. Security Considerations

- **Nonce uniqueness**: Challenge nonces MUST be UUID v4 values generated with a CSPRNG. The service MUST track issued nonces and reject any nonce that has already been used.
- **Session fixation**: Session IDs are generated server-side and cannot be set by the client. Sessions are bound to the agent DID established during Phase 1.
- **Payment token binding**: The payment token's `sub` claim must match the session's agent DID, preventing token theft across sessions.
- **Terms integrity**: The `termsHash` mechanism ensures the agent cannot be bait-and-switched with different terms between negotiation and payment.
- **Timeout enforcement**: The 30-minute session TTL prevents abandoned sessions from accumulating state indefinitely.
