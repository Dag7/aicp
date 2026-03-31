# AICP Discovery Layer Specification

**Version:** 1.0.0
**Status:** Draft
**Last Updated:** 2026-03-30

---

## 1. Overview

The Discovery Layer enables AI agents to find and evaluate services programmatically. Built around the Agent Name System (ANS), it provides a structured, machine-readable directory of AICP-compatible services. Discovery operates across three tiers --- local cache, domain well-known files, and a global registry --- to balance performance with freshness.

---

## 2. Agent Name System (ANS)

### 2.1 Purpose

The ANS provides a decentralized, DNS-inspired naming system for AICP services. It allows AI agents to:

- Discover services by capability, category, or tag.
- Retrieve structured metadata for automated onboarding.
- Evaluate trustworthiness before initiating commerce.

### 2.2 Design Goals

1. **Machine-first**: Records are structured JSON designed for automated parsing, not human browsing.
2. **Decentralized discovery**: Any domain can publish an ANS record without registering with a central authority.
3. **Trust-layered**: Trust signals are embedded in records and verified independently.
4. **Cache-friendly**: Records include version identifiers and TTL hints for efficient caching.

---

## 3. ANS Record Format

Each AICP-compatible service publishes a JSON record at `/.well-known/aicp.json` on its domain.

### 3.1 Complete Record Schema

```json
{
  "version": "1.0",
  "service": {
    "did": "did:aicp:9XyZaBcDeFgHiJkLmNoPqR",
    "name": "CloudInference Pro",
    "description": "High-performance AI inference service with GPU-accelerated models",
    "category": "ai-inference",
    "tags": ["llm", "image-generation", "embeddings", "gpu"],
    "logo_url": "https://cloudinference.example.com/logo.png",
    "terms_url": "https://cloudinference.example.com/terms",
    "privacy_url": "https://cloudinference.example.com/privacy"
  },
  "onboarding": {
    "protocol_version": "1.0",
    "required_credentials": [
      "AICPAgentDelegation",
      "AICPSpendingAuthorization"
    ],
    "minimum_trust_score": 200,
    "supported_payment_adapters": ["stripe", "usdc"],
    "estimated_onboarding_time_seconds": 15
  },
  "authentication": {
    "methods": ["did-auth"],
    "challenge_endpoint": "/aicp/v1/auth/challenge",
    "session_ttl_seconds": 1800
  },
  "pricing": {
    "model": "usage",
    "currency": "USD",
    "tiers": [
      {
        "name": "standard",
        "description": "Standard inference tier",
        "unit": "request",
        "unit_price": "0.002",
        "rate_limit": 1000
      },
      {
        "name": "premium",
        "description": "Priority inference with dedicated GPU",
        "unit": "request",
        "unit_price": "0.01",
        "rate_limit": 10000
      }
    ],
    "free_tier": {
      "requests": 100,
      "period": "day"
    }
  },
  "trust": {
    "verified_by": [
      {
        "did": "did:aicp:ConsortiumVerifier1abc",
        "name": "AICP Foundation",
        "verified_at": "2026-01-10T00:00:00Z"
      }
    ],
    "established_since": "2025-06-01T00:00:00Z",
    "certifications": [
      "soc2-type2",
      "gdpr-compliant",
      "aicp-verified"
    ],
    "uptime_sla": "99.9%"
  },
  "endpoints": {
    "onboard": "https://cloudinference.example.com/aicp/v1/onboard",
    "api": "https://cloudinference.example.com/aicp/v1",
    "websocket": "wss://cloudinference.example.com/aicp/v1/ws",
    "health": "https://cloudinference.example.com/aicp/v1/health"
  },
  "discovery_tags": [
    "machine-learning",
    "natural-language-processing",
    "computer-vision",
    "real-time-inference",
    "batch-processing"
  ]
}
```

### 3.2 Field Reference

#### `service` (required)

| Field | Type | Required | Description |
|---|---|---|---|
| `did` | string | Yes | The service's AICP DID |
| `name` | string | Yes | Human-readable service name |
| `description` | string | Yes | Brief description of the service (max 500 chars) |
| `category` | string | Yes | Primary service category (see Section 3.3) |
| `tags` | string[] | No | Additional descriptive tags |
| `logo_url` | string | No | URL to service logo |
| `terms_url` | string | No | URL to terms of service |
| `privacy_url` | string | No | URL to privacy policy |

#### `onboarding` (required)

| Field | Type | Required | Description |
|---|---|---|---|
| `protocol_version` | string | Yes | AICP protocol version supported |
| `required_credentials` | string[] | Yes | VC types required for onboarding |
| `minimum_trust_score` | integer | No | Minimum trust score to onboard (0-1000) |
| `supported_payment_adapters` | string[] | Yes | Accepted payment adapters |
| `estimated_onboarding_time_seconds` | integer | No | Estimated time to complete onboarding |

#### `authentication` (required)

| Field | Type | Required | Description |
|---|---|---|---|
| `methods` | string[] | Yes | Supported auth methods (currently `did-auth`) |
| `challenge_endpoint` | string | Yes | Relative path for DID-Auth challenges |
| `session_ttl_seconds` | integer | No | Session duration (default: 1800) |

#### `pricing` (required)

| Field | Type | Required | Description |
|---|---|---|---|
| `model` | string | Yes | Pricing model: `usage`, `subscription`, `one-time`, `tiered` |
| `currency` | string | Yes | ISO 4217 currency code |
| `tiers` | object[] | Yes | Pricing tier definitions |
| `free_tier` | object | No | Free usage allowance |

#### `trust` (optional but recommended)

| Field | Type | Required | Description |
|---|---|---|---|
| `verified_by` | object[] | No | Third-party verifiers |
| `established_since` | string | No | ISO 8601 date of service establishment |
| `certifications` | string[] | No | Certification identifiers |
| `uptime_sla` | string | No | Uptime commitment |

#### `endpoints` (required)

| Field | Type | Required | Description |
|---|---|---|---|
| `onboard` | string | Yes | Full URL for onboarding initiation |
| `api` | string | Yes | Base URL for REST API |
| `websocket` | string | No | WebSocket URL for real-time events |
| `health` | string | No | Health check endpoint |

#### `discovery_tags` (optional)

An array of free-form strings used for full-text search indexing. Maximum 20 tags, each no longer than 50 characters.

### 3.3 Service Categories

The following categories are defined in the protocol:

| Category | Description |
|---|---|
| `ai-inference` | AI model inference services |
| `compute` | General compute resources |
| `storage` | Data storage services |
| `data` | Data feeds and APIs |
| `communication` | Messaging and notification services |
| `financial` | Financial and payment services |
| `identity` | Identity verification services |
| `analytics` | Analytics and monitoring |
| `development` | Developer tools and CI/CD |
| `other` | Uncategorized services |

---

## 4. Resolution Tiers

ANS resolution follows a tiered lookup strategy, proceeding from fastest to most authoritative.

### 4.1 Tier 1: Local Cache

The agent's local ANS cache (backed by Redis) is checked first.

```
Cache Key:    ans:record:<domain>
Cache TTL:    Determined by the record's cache control (default: 300 seconds)
```

If a valid cached record exists, it is returned immediately.

### 4.2 Tier 2: Domain Well-Known

If the cache misses, the agent fetches `https://<domain>/.well-known/aicp.json` directly.

**Request:**

```http
GET /.well-known/aicp.json HTTP/1.1
Host: cloudinference.example.com
Accept: application/json
```

**Validation:**

1. The response must be valid JSON conforming to the ANS record schema.
2. The `service.did` must resolve to a valid, non-deactivated DID Document.
3. The DID Document's controller domain must match the serving domain (domain binding).

On success, the record is cached at Tier 1.

### 4.3 Tier 3: Global ANS Registry

If the domain well-known file is unavailable or the domain is unknown, the agent queries the global ANS registry maintained by the consortium.

```http
GET /aicp/v1/ans/records?domain=cloudinference.example.com
Host: registry.aicp.dev
Accept: application/json
```

The registry maintains a database of all registered ANS records, indexed for full-text search.

### 4.4 Resolution Order

```
Agent Request
    |
    v
[Tier 1: Local Cache] --hit--> Return Record
    |
    miss
    v
[Tier 2: Domain Well-Known] --success--> Cache & Return
    |
    failure
    v
[Tier 3: Global Registry] --found--> Cache & Return
    |
    not found
    v
Return NOT_FOUND error
```

---

## 5. Search

### 5.1 Capability-Based Search

Agents search for services by capability, category, or free-text query.

```http
GET /aicp/v1/ans/search?q=gpu+inference&category=ai-inference&min_trust=300
Host: registry.aicp.dev
Accept: application/json
```

**Query Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `q` | string | Free-text search query |
| `category` | string | Filter by service category |
| `tags` | string | Comma-separated tag filter |
| `min_trust` | integer | Minimum trust score (0-1000) |
| `payment_adapter` | string | Filter by supported payment adapter |
| `limit` | integer | Results per page (default: 20, max: 100) |
| `cursor` | string | Pagination cursor |

### 5.2 Full-Text Search Implementation

Search is implemented using PostgreSQL full-text search with `tsvector` indexing across:

- `service.name` (weight A)
- `service.description` (weight B)
- `service.tags` (weight C)
- `discovery_tags` (weight C)

The search query is parsed into a `tsquery` and matched against the composite `tsvector`.

### 5.3 Relevance Ranking

Results are ranked by a composite score combining text relevance and trust:

```
final_score = (0.6 * text_relevance) + (0.4 * normalized_trust_score)
```

Where:
- `text_relevance` is the PostgreSQL `ts_rank_cd` score, normalized to [0, 1].
- `normalized_trust_score` is the service's trust score divided by 1000.

### 5.4 Search Response

```json
{
  "data": {
    "results": [
      {
        "domain": "cloudinference.example.com",
        "service": {
          "did": "did:aicp:9XyZaBcDeFgHiJkLmNoPqR",
          "name": "CloudInference Pro",
          "description": "High-performance AI inference service",
          "category": "ai-inference"
        },
        "trust_score": 780,
        "relevance_score": 0.92,
        "onboard_url": "https://cloudinference.example.com/aicp/v1/onboard"
      }
    ],
    "next_cursor": "eyJvZmZzZXQiOjIwfQ==",
    "has_more": true,
    "total_estimate": 47
  }
}
```

---

## 6. Trust Signals

### 6.1 Verified By

The `trust.verified_by` array lists third-party entities that have verified the service. Each entry includes the verifier's DID, name, and verification date. Agents can resolve the verifier's DID and check its own trust signals to assess the verification's weight.

### 6.2 Established Since

The `trust.established_since` field indicates when the service was first registered. Older services accumulate more trust through transaction history.

### 6.3 Certifications

Certification identifiers reference externally auditable standards:

| Certification | Description |
|---|---|
| `aicp-verified` | Verified by the AICP Foundation |
| `soc2-type2` | SOC 2 Type II audit completed |
| `gdpr-compliant` | GDPR compliance verified |
| `iso27001` | ISO 27001 certified |
| `pci-dss` | PCI DSS compliant |

---

## 7. Endpoint Discovery

### 7.1 Locating Service Endpoints

After resolving an ANS record, agents use the `endpoints` block to locate service URLs:

1. **Onboarding**: `endpoints.onboard` --- the entry point for the 4-phase onboarding flow.
2. **API access**: `endpoints.api` --- the base URL for authenticated REST API calls.
3. **Real-time events**: `endpoints.websocket` --- the WebSocket endpoint for event subscriptions.
4. **Health check**: `endpoints.health` --- used to verify service availability before onboarding.

### 7.2 Health Check

Before initiating onboarding, agents SHOULD verify the service is healthy:

```http
GET /aicp/v1/health
Host: cloudinference.example.com
```

**Response:**

```json
{
  "data": {
    "status": "healthy",
    "version": "1.0.0",
    "timestamp": "2026-03-30T12:00:00Z"
  }
}
```

Valid status values: `healthy`, `degraded`, `unhealthy`.

---

## 8. Caching

### 8.1 TTL-Based Cache

ANS records are cached in Redis with a configurable TTL.

| Cache Key Pattern | Default TTL | Description |
|---|---|---|
| `ans:record:<domain>` | 300s | Full ANS record |
| `ans:search:<query_hash>` | 60s | Search result set |
| `ans:health:<domain>` | 30s | Health check result |

### 8.2 Cache Invalidation

Caches are invalidated in the following cases:

1. **TTL expiry**: Record is evicted and re-fetched on next access.
2. **Event-driven**: When the global registry receives an ANS record update, it publishes an `ans.record.updated` event via WebSocket. Subscribed agents invalidate their local cache for the affected domain.
3. **Manual purge**: Agents can force-refresh a record by passing `Cache-Control: no-cache` on the resolution request.

### 8.3 Stale-While-Revalidate

If a cached record's TTL has expired but the origin is temporarily unreachable, the agent MAY serve the stale record for up to 5 minutes while attempting revalidation in the background. The stale response must be flagged:

```json
{
  "data": { ... },
  "meta": {
    "cache": "stale",
    "cached_at": "2026-03-30T11:55:00Z",
    "revalidating": true
  }
}
```

---

## 9. Security Considerations

- **Domain binding**: The `service.did` in an ANS record must be verifiably bound to the serving domain. Implementations MUST verify that the DID Document includes the domain in its `service` or `alsoKnownAs` property.
- **TLS requirement**: ANS records MUST be served over HTTPS. Agents MUST reject records served over plain HTTP.
- **Record tampering**: Agents SHOULD verify that the ANS record is signed by the service's DID when fetching from the global registry. Domain well-known records derive their trust from the TLS certificate binding.
- **Search poisoning**: The global registry MUST validate ANS records against their source domain before indexing to prevent impersonation.
