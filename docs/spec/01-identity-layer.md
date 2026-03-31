# AICP Identity Layer Specification

**Version:** 1.0.0
**Status:** Draft
**Last Updated:** 2026-03-30

---

## 1. Overview

The Identity Layer is the foundation of the AI Commerce Protocol. It provides decentralized, self-sovereign identity for both human principals and their AI agents using W3C Decentralized Identifiers (DIDs) and Verifiable Credentials (VCs). Every participant in the AICP ecosystem --- principals, agents, and services --- is identified by a DID and authenticates using Ed25519 cryptographic signatures.

---

## 2. DID Method: `did:aicp`

### 2.1 Method Syntax

The `did:aicp` method defines two identifier forms:

```
did:aicp:<principal-id>
did:aicp:<principal-id>:<agent-id>
```

**Principal DID** identifies a human or organizational entity:

```
did:aicp:7Hk3dQwPvLmNxR9sTuVwXy
```

**Agent DID** identifies an AI agent operating under a principal's authority:

```
did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx
```

### 2.2 Identifier Derivation

Identifiers are derived from Ed25519 public keys using the following procedure:

1. Generate an Ed25519 keypair.
2. Compute SHA-256 hash of the 32-byte public key.
3. Truncate the hash to 16 bytes (128 bits).
4. Encode the truncated hash using base58btc (Bitcoin alphabet).

```
publicKey     = Ed25519.GenerateKey()
hash          = SHA-256(publicKey)
truncated     = hash[0:16]
identifier    = base58btc.Encode(truncated)
```

The resulting identifier is 22 characters long on average. The 128-bit truncation provides collision resistance of approximately 2^64 under the birthday bound, which is sufficient for the anticipated namespace size.

### 2.3 ABNF Grammar

```abnf
did-aicp       = "did:aicp:" principal-id [ ":" agent-id ]
principal-id   = 1*22base58btc-char
agent-id       = 1*22base58btc-char
base58btc-char = "1" / "2" / "3" / "4" / "5" / "6" / "7" / "8" / "9"
               / "A" / "B" / "C" / "D" / "E" / "F" / "G" / "H" / "J"
               / "K" / "L" / "M" / "N" / "P" / "Q" / "R" / "S" / "T"
               / "U" / "V" / "W" / "X" / "Y" / "Z"
               / "a" / "b" / "c" / "d" / "e" / "f" / "g" / "h" / "i"
               / "j" / "k" / "m" / "n" / "o" / "p" / "q" / "r" / "s"
               / "t" / "u" / "v" / "w" / "x" / "y" / "z"
```

---

## 3. DID Document Structure

DID Documents follow the W3C DID Core v1.0 specification and are serialized as JSON-LD.

### 3.1 Principal DID Document

```json
{
  "@context": [
    "https://www.w3.org/ns/did/v1",
    "https://w3id.org/security/suites/ed25519-2020/v1",
    "https://aicp.dev/ns/v1"
  ],
  "id": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy",
  "created": "2026-01-15T10:30:00Z",
  "updated": "2026-01-15T10:30:00Z",
  "version": 1,
  "verificationMethod": [
    {
      "id": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy#key-1",
      "type": "Ed25519VerificationKey2020",
      "controller": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy",
      "publicKeyMultibase": "z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK"
    }
  ],
  "authentication": [
    "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy#key-1"
  ],
  "assertionMethod": [
    "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy#key-1"
  ],
  "capabilityDelegation": [
    "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy#key-1"
  ]
}
```

### 3.2 Agent DID Document

```json
{
  "@context": [
    "https://www.w3.org/ns/did/v1",
    "https://w3id.org/security/suites/ed25519-2020/v1",
    "https://aicp.dev/ns/v1"
  ],
  "id": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx",
  "controller": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy",
  "created": "2026-01-15T11:00:00Z",
  "updated": "2026-01-15T11:00:00Z",
  "version": 1,
  "verificationMethod": [
    {
      "id": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx#key-1",
      "type": "Ed25519VerificationKey2020",
      "controller": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx",
      "publicKeyMultibase": "z6MkpTHR8VNsBxYAAWHut2Geadd9jSwuBV8xRoAnwWsdvktH"
    }
  ],
  "authentication": [
    "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx#key-1"
  ],
  "assertionMethod": [
    "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx#key-1"
  ]
}
```

### 3.3 Verification Relationships

| Relationship | Principal | Agent | Purpose |
|---|---|---|---|
| `authentication` | Yes | Yes | DID-Auth challenge-response |
| `assertionMethod` | Yes | Yes | Signing VCs and payment tokens |
| `capabilityDelegation` | Yes | No | Delegating capabilities to agents |

Agents do not have `capabilityDelegation` --- they cannot sub-delegate. Only principals may delegate.

---

## 4. DID Lifecycle

### 4.1 Create

A new DID is created by generating an Ed25519 keypair, deriving the identifier, constructing the DID Document, and anchoring it to the consortium ledger.

**Request:**

```http
POST /aicp/v1/dids
Content-Type: application/json

{
  "didDocument": { ... },
  "proof": {
    "type": "Ed25519Signature2020",
    "created": "2026-01-15T10:30:00Z",
    "verificationMethod": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy#key-1",
    "proofPurpose": "authentication",
    "proofValue": "z58DAdFfa9SkqZMVPxAQpic7ndTn..."
  }
}
```

The ledger validates that:
- The DID does not already exist.
- The proof signature is valid for the public key in the document.
- The DID identifier matches the public key derivation.

### 4.2 Resolve

Resolution retrieves the current DID Document for a given DID.

```http
GET /aicp/v1/dids/did:aicp:7Hk3dQwPvLmNxR9sTuVwXy
Accept: application/json
```

**Response:**

```json
{
  "data": {
    "didDocument": { ... },
    "didDocumentMetadata": {
      "created": "2026-01-15T10:30:00Z",
      "updated": "2026-01-15T10:30:00Z",
      "version": 1,
      "deactivated": false
    }
  }
}
```

### 4.3 Update

Updates are version-tracked. Each update increments the `version` field and must be signed by a key referenced in the current document's `authentication` relationship.

```http
PUT /aicp/v1/dids/did:aicp:7Hk3dQwPvLmNxR9sTuVwXy
Content-Type: application/json

{
  "didDocument": { ... },
  "previousVersion": 1,
  "proof": { ... }
}
```

The `previousVersion` field provides optimistic concurrency control. The update is rejected if the current version does not match.

### 4.4 Deactivate

Deactivation is a soft-delete. The DID Document is retained but marked as deactivated. Deactivated DIDs cannot authenticate or issue credentials, but existing credentials can still be verified against the historical document.

```http
DELETE /aicp/v1/dids/did:aicp:7Hk3dQwPvLmNxR9sTuVwXy

{
  "proof": { ... }
}
```

After deactivation, resolution returns the document with `"deactivated": true` in metadata.

---

## 5. Key Management

### 5.1 Key Independence

Each agent holds its own Ed25519 keypair, independent of its principal's keys. Compromise of an agent key does not compromise the principal or other agents.

### 5.2 Key Encoding

Public keys are encoded using the multibase format with the `z` prefix (base58btc):

```
z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK
```

The `z` prefix signals base58btc encoding. The first two bytes after decoding are the multicodec identifier for Ed25519 public keys (`0xed01`).

### 5.3 Key Rotation

Key rotation allows a DID holder to replace their active key without changing their DID.

**Procedure:**

1. Generate a new Ed25519 keypair.
2. Add the new key to the DID Document's `verificationMethod` array.
3. Update `authentication` (and other relationships) to reference the new key.
4. Sign the update with the **old** key.
5. Submit the update to the ledger.

**Grace Period:**

After rotation, the old key remains valid for authentication for a configurable grace period (default: 72 hours). This allows in-flight operations signed with the old key to complete. During the grace period, both old and new keys appear in the document:

```json
{
  "verificationMethod": [
    {
      "id": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy#key-1",
      "type": "Ed25519VerificationKey2020",
      "controller": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy",
      "publicKeyMultibase": "z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
      "revoked": "2026-03-01T12:00:00Z",
      "graceExpiry": "2026-03-04T12:00:00Z"
    },
    {
      "id": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy#key-2",
      "type": "Ed25519VerificationKey2020",
      "controller": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy",
      "publicKeyMultibase": "z6MkpTHR8VNsBxYAAWHut2Geadd9jSwuBV8xRoAnwWsdvktH"
    }
  ],
  "authentication": [
    "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy#key-2"
  ]
}
```

After the grace period expires, the old key is removed from the document on the next update.

---

## 6. Principal-Agent Delegation

### 6.1 Delegation Model

Principals create agent DIDs and issue `AICPAgentDelegation` Verifiable Credentials to define the agent's authority. The delegation VC specifies:

- Which capabilities the agent may exercise.
- Constraints on those capabilities (spending limits, service categories, geographies).
- The validity period of the delegation.

### 6.2 Agent DID Creation

A principal creates an agent DID by:

1. Generating a new Ed25519 keypair for the agent.
2. Constructing the agent DID Document with the principal's DID as `controller`.
3. Signing the creation request with the principal's `capabilityDelegation` key.
4. Anchoring the agent DID Document to the ledger.

### 6.3 Delegation Constraints

Constraints are expressed in the `credentialSubject` of the delegation VC:

```json
{
  "credentialSubject": {
    "id": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx",
    "delegatedCapabilities": [
      "service-onboarding",
      "subscription-payment",
      "usage-payment"
    ],
    "constraints": {
      "maxTransactionAmount": {
        "value": "100.00",
        "currency": "USD"
      },
      "dailySpendingLimit": {
        "value": "500.00",
        "currency": "USD"
      },
      "monthlySpendingLimit": {
        "value": "5000.00",
        "currency": "USD"
      },
      "allowedServiceCategories": [
        "compute",
        "storage",
        "ai-inference"
      ],
      "allowedGeographies": [
        "US",
        "EU",
        "GB"
      ]
    }
  }
}
```

---

## 7. Verifiable Credentials

All credentials follow the W3C Verifiable Credentials Data Model 1.1.

### 7.1 AICPAgentDelegation

Issued by a principal to an agent to delegate capabilities.

```json
{
  "@context": [
    "https://www.w3.org/2018/credentials/v1",
    "https://aicp.dev/ns/credentials/v1"
  ],
  "id": "urn:uuid:3e671c42-a4f8-4b90-b5d3-8c1a4e7f2d9b",
  "type": ["VerifiableCredential", "AICPAgentDelegation"],
  "issuer": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy",
  "issuanceDate": "2026-01-15T11:00:00Z",
  "expirationDate": "2027-01-15T11:00:00Z",
  "credentialSubject": {
    "id": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx",
    "delegatedCapabilities": [
      "service-onboarding",
      "subscription-payment",
      "usage-payment"
    ],
    "constraints": {
      "maxTransactionAmount": { "value": "100.00", "currency": "USD" },
      "dailySpendingLimit": { "value": "500.00", "currency": "USD" },
      "monthlySpendingLimit": { "value": "5000.00", "currency": "USD" },
      "allowedServiceCategories": ["compute", "storage", "ai-inference"],
      "allowedGeographies": ["US", "EU"]
    }
  },
  "credentialStatus": {
    "id": "https://registry.aicp.dev/credentials/status/3#42",
    "type": "RevocationList2020Status",
    "revocationListIndex": "42",
    "revocationListCredential": "https://registry.aicp.dev/credentials/status/3"
  },
  "proof": {
    "type": "Ed25519Signature2020",
    "created": "2026-01-15T11:00:00Z",
    "verificationMethod": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy#key-1",
    "proofPurpose": "assertionMethod",
    "proofValue": "z58DAdFfa9SkqZMVPxAQpic7ndTn..."
  }
}
```

### 7.2 AICPSpendingAuthorization

Issued by a principal to authorize an agent to use a specific payment method.

```json
{
  "@context": [
    "https://www.w3.org/2018/credentials/v1",
    "https://aicp.dev/ns/credentials/v1"
  ],
  "id": "urn:uuid:a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "type": ["VerifiableCredential", "AICPSpendingAuthorization"],
  "issuer": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy",
  "issuanceDate": "2026-01-15T11:05:00Z",
  "expirationDate": "2027-01-15T11:05:00Z",
  "credentialSubject": {
    "id": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx",
    "paymentMethod": {
      "adapterId": "stripe",
      "tokenReference": "pm_encrypted_ref_abc123"
    },
    "limits": {
      "maxTransactionAmount": { "value": "100.00", "currency": "USD" },
      "dailyLimit": { "value": "500.00", "currency": "USD" },
      "monthlyLimit": { "value": "5000.00", "currency": "USD" }
    },
    "allowedPurposes": [
      "service-onboarding",
      "subscription-payment",
      "usage-payment"
    ]
  },
  "credentialStatus": {
    "id": "https://registry.aicp.dev/credentials/status/4#17",
    "type": "RevocationList2020Status",
    "revocationListIndex": "17",
    "revocationListCredential": "https://registry.aicp.dev/credentials/status/4"
  },
  "proof": { ... }
}
```

### 7.3 AICPServiceAttestation

Issued by a service to endorse another service or agent after a successful interaction.

```json
{
  "@context": [
    "https://www.w3.org/2018/credentials/v1",
    "https://aicp.dev/ns/credentials/v1"
  ],
  "id": "urn:uuid:f1e2d3c4-b5a6-0987-fedc-ba9876543210",
  "type": ["VerifiableCredential", "AICPServiceAttestation"],
  "issuer": "did:aicp:9XyZaBcDeFgHiJkLmNoPqR",
  "issuanceDate": "2026-02-20T14:30:00Z",
  "credentialSubject": {
    "id": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx",
    "attestationType": "successful-interaction",
    "serviceCategory": "ai-inference",
    "interactionCount": 47,
    "period": {
      "from": "2026-01-15T00:00:00Z",
      "to": "2026-02-15T00:00:00Z"
    },
    "rating": 4.8
  },
  "proof": { ... }
}
```

---

## 8. Revocation

### 8.1 RevocationList2020

Credential revocation uses the RevocationList2020 mechanism. Each revocation list is itself a Verifiable Credential containing a compressed bitstring where each bit position corresponds to a credential index.

```json
{
  "@context": [
    "https://www.w3.org/2018/credentials/v1",
    "https://w3id.org/vc-revocation-list-2020/v1"
  ],
  "id": "https://registry.aicp.dev/credentials/status/3",
  "type": ["VerifiableCredential", "RevocationList2020Credential"],
  "issuer": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy",
  "issuanceDate": "2026-01-15T10:00:00Z",
  "credentialSubject": {
    "id": "https://registry.aicp.dev/credentials/status/3#list",
    "type": "RevocationList2020",
    "encodedList": "H4sIAAAAAAAAA-3BMQEAAAACIGf_0TYRAAAAAAAAAAAAAAAAAAA..."
  },
  "proof": { ... }
}
```

### 8.2 Bitstring Compression

The bitstring is GZIP-compressed and base64-encoded. The minimum bitstring size is 16KB (131,072 bit positions), providing privacy by herd through a large anonymity set.

### 8.3 Credential Status Checking

To check if a credential is revoked:

1. Retrieve the `credentialStatus` from the credential.
2. Fetch the `revocationListCredential` URL.
3. Verify the revocation list credential's proof.
4. Decode and decompress the `encodedList` bitstring.
5. Check the bit at the position specified by `revocationListIndex`.
6. If the bit is `1`, the credential is revoked.

Implementations SHOULD cache revocation lists with a TTL of no more than 5 minutes.

---

## 9. Proofs

### 9.1 Ed25519Signature2020

All proofs in the AICP protocol use the Ed25519Signature2020 suite.

**Proof Structure:**

```json
{
  "type": "Ed25519Signature2020",
  "created": "2026-01-15T11:00:00Z",
  "verificationMethod": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy#key-1",
  "proofPurpose": "assertionMethod",
  "proofValue": "z58DAdFfa9SkqZMVPxAQpic7ndTn3Uhtya4fMwQSjUd8MqzJ...",
  "challenge": "9a4f2c8e-3b7d-4a1f-b5e2-1c8d9f0a3b6e",
  "domain": "https://service.example.com"
}
```

### 9.2 Proof Fields

| Field | Required | Description |
|---|---|---|
| `type` | Yes | Always `Ed25519Signature2020` |
| `created` | Yes | ISO 8601 timestamp of proof creation |
| `verificationMethod` | Yes | DID URL of the signing key |
| `proofPurpose` | Yes | One of: `authentication`, `assertionMethod`, `capabilityDelegation` |
| `proofValue` | Yes | Multibase-encoded (z-prefix base58btc) signature bytes |
| `challenge` | Conditional | Required for `authentication` proofs (DID-Auth). A server-issued nonce. |
| `domain` | Conditional | Required for `authentication` proofs. The domain of the verifying party. |

### 9.3 Proof Verification

To verify a proof:

1. Resolve the DID in `verificationMethod` to obtain the DID Document.
2. Confirm the referenced key exists in the document.
3. Confirm the key is listed under the relationship matching `proofPurpose`.
4. If `challenge` is present, verify it matches the expected server-issued nonce.
5. If `domain` is present, verify it matches the verifying party's domain.
6. Canonicalize the document (excluding the `proof` property) using JSON Canonicalization Scheme (JCS, RFC 8785).
7. Verify the Ed25519 signature in `proofValue` against the canonicalized document using the resolved public key.

---

## 10. Security Considerations

- **Key storage**: Private keys MUST be stored in secure enclaves or hardware security modules in production deployments. Software-based key storage is acceptable only for development.
- **Identifier collision**: The 128-bit truncation provides adequate collision resistance. Implementations MUST check for collisions during DID creation and reject duplicates.
- **Replay attacks**: Authentication proofs MUST include a `challenge` (nonce) and `domain` to prevent replay across services or sessions.
- **Key compromise**: If an agent key is compromised, the principal SHOULD immediately revoke the agent's delegation credential and deactivate the agent DID. The principal's own keys are unaffected.
- **Proof timestamps**: Verifiers SHOULD reject proofs with `created` timestamps more than 5 minutes in the past to limit replay windows.
