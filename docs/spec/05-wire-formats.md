# AICP Wire Formats Specification

**Version:** 1.0.0
**Status:** Draft
**Last Updated:** 2026-03-30

---

## 1. Overview

This document specifies the wire formats for communication between AICP protocol participants. The protocol uses HTTP/REST for request-response interactions and WebSocket for real-time event streaming. All payloads are JSON-encoded.

---

## 2. HTTP/REST API

### 2.1 Base URL Pattern

All AICP endpoints are served under a versioned prefix:

```
https://<host>/aicp/v1/
```

Examples:

```
https://cloudinference.example.com/aicp/v1/dids
https://cloudinference.example.com/aicp/v1/onboard/initiate
https://registry.aicp.dev/aicp/v1/ans/search
```

### 2.2 Content Type

All requests and responses use `application/json`:

```
Content-Type: application/json
Accept: application/json
```

Payment tokens use the specialized content type `aicp-payment-token+jws` within the JWS `typ` header, but are transmitted inside a JSON request body.

---

## 3. Response Envelope

### 3.1 Success Response

All successful responses wrap their payload in a `data` field:

```json
{
  "data": {
    "id": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy",
    "created": "2026-01-15T10:30:00Z"
  }
}
```

For paginated responses, the `data` object contains the results along with pagination metadata:

```json
{
  "data": {
    "results": [ ... ],
    "next_cursor": "eyJvZmZzZXQiOjIwfQ==",
    "has_more": true
  }
}
```

### 3.2 Error Response

All error responses wrap their payload in an `error` field:

```json
{
  "error": {
    "code": "DID_NOT_FOUND",
    "message": "The specified DID was not found",
    "detail": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy could not be resolved"
  }
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `code` | string | Yes | Machine-readable error code (see 06-error-codes.md) |
| `message` | string | Yes | Human-readable error description |
| `detail` | string | No | Additional context about the error |

### 3.3 Metadata

Responses MAY include an optional `meta` field for non-essential metadata:

```json
{
  "data": { ... },
  "meta": {
    "requestId": "req_a1b2c3d4e5f6",
    "processingTime": 42,
    "cache": "hit"
  }
}
```

---

## 4. HTTP Status Codes

AICP maps protocol errors to standard HTTP status codes:

| HTTP Status | Usage | Protocol Error Codes |
|---|---|---|
| 200 OK | Successful retrieval or operation | --- |
| 201 Created | Resource successfully created | --- |
| 204 No Content | Successful deletion | --- |
| 400 Bad Request | Malformed request or invalid parameters | `BAD_REQUEST`, `INVALID_PHASE` |
| 401 Unauthorized | Missing or invalid authentication | `UNAUTHORIZED` |
| 402 Payment Required | Payment processing failed | `PAYMENT_FAILED` |
| 403 Forbidden | Insufficient permissions or revoked credentials | `FORBIDDEN`, `CREDENTIAL_REVOKED`, `CREDENTIAL_EXPIRED`, `PAYMENT_TOKEN_EXPIRED`, `SPENDING_LIMIT_EXCEEDED` |
| 404 Not Found | Resource does not exist | `NOT_FOUND`, `DID_NOT_FOUND` |
| 409 Conflict | Resource already exists or duplicate operation | `CONFLICT`, `PAYMENT_TOKEN_REPLAY` |
| 410 Gone | Resource has been deactivated or expired | `DID_DEACTIVATED`, `SESSION_EXPIRED` |
| 429 Too Many Requests | Rate limit exceeded | `RATE_LIMITED` |
| 500 Internal Server Error | Unexpected server error | `INTERNAL_ERROR` |

---

## 5. Authentication

### 5.1 DID-Auth Header

For DID-authenticated requests, the `Authorization` header uses the `DID` scheme:

```
Authorization: DID <did> <signature>
```

Where:
- `<did>` is the full DID of the authenticating entity.
- `<signature>` is a base58btc-encoded Ed25519 signature over the canonical request representation.

**Canonical request representation:**

```
method + "\n" + path + "\n" + timestamp + "\n" + body_hash
```

Where:
- `method` is the HTTP method (uppercase).
- `path` is the request path including query string.
- `timestamp` is the value of the `X-AICP-Timestamp` header (ISO 8601).
- `body_hash` is the SHA-256 hex hash of the request body (or empty string if no body).

**Example:**

```http
POST /aicp/v1/onboard/initiate HTTP/1.1
Host: cloudinference.example.com
Content-Type: application/json
Authorization: DID did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx z4Kj7V9eQ3pR2sT...
X-AICP-Timestamp: 2026-03-30T12:00:00Z

{"agentDid": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx"}
```

**Verification:**

1. Extract the DID and signature from the header.
2. Resolve the DID to obtain the public key.
3. Reconstruct the canonical request representation.
4. Verify the Ed25519 signature.
5. Confirm the timestamp is within 5 minutes of the server's clock.

### 5.2 Bearer Token

For authenticated sessions (post-onboarding), standard Bearer authentication is used:

```
Authorization: Bearer aicp_live_k3y_a1b2c3d4e5f6g7h8i9j0
```

Bearer tokens are issued during the provisioning phase and are bound to the agent DID.

### 5.3 Authentication Precedence

If both `DID` and `Bearer` authentication are present, the server MUST use `DID` authentication. `Bearer` is a convenience for post-onboarding API calls.

---

## 6. WebSocket Protocol

### 6.1 Connection Establishment

WebSocket connections use ticket-based authentication to avoid exposing credentials in the URL.

**Step 1: Obtain a ticket**

```http
POST /aicp/v1/ws/ticket
Authorization: DID did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx z4Kj7V9eQ3pR2sT...
X-AICP-Timestamp: 2026-03-30T12:00:00Z
```

**Response:**

```json
{
  "data": {
    "ticket": "wst_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
    "expiresAt": "2026-03-30T12:01:00Z"
  }
}
```

Tickets are single-use and expire after 60 seconds.

**Step 2: Connect with ticket**

```
GET /aicp/v1/ws?ticket=wst_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
Upgrade: websocket
Connection: Upgrade
```

The server validates the ticket, associates the WebSocket connection with the agent DID, and completes the upgrade.

### 6.2 Event Format

All WebSocket messages are JSON objects with the following envelope:

```json
{
  "eventId": "evt_a1b2c3d4e5f6",
  "type": "payment.captured",
  "timestamp": "2026-03-30T12:05:00Z",
  "source": "did:aicp:9XyZaBcDeFgHiJkLmNoPqR",
  "subject": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy:3FgHjKlMnPqRsT5vWx",
  "data": {
    "receiptId": "rcpt_m1n2o3p4q5r6",
    "amount": "25.00",
    "currency": "USD"
  },
  "signature": "z4Kj7V9eQ3pR2sT..."
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `eventId` | string | Yes | Unique event identifier. Format: `evt_` + 12 alphanumeric chars. |
| `type` | string | Yes | Dot-delimited event type (e.g., `payment.captured`). |
| `timestamp` | string | Yes | ISO 8601 timestamp of the event. |
| `source` | string | Yes | DID of the entity that generated the event. |
| `subject` | string | No | DID of the entity the event pertains to. |
| `data` | object | Yes | Event-specific payload. |
| `signature` | string | No | Ed25519 signature over the canonical event (eventId + type + timestamp + source + JSON(data)). |

### 6.3 Event Types

| Type | Description |
|---|---|
| `did.created` | A new DID was created |
| `did.updated` | A DID Document was updated |
| `did.deactivated` | A DID was deactivated |
| `onboard.session.initiated` | Onboarding session started |
| `onboard.session.identity_presented` | Agent identity verified |
| `onboard.session.capabilities_negotiated` | Capabilities agreed upon |
| `onboard.session.payment_authorized` | Payment authorized |
| `onboard.session.provisioned` | Account provisioned |
| `onboard.session.failed` | Onboarding failed |
| `payment.intent_created` | Payment intent created |
| `payment.captured` | Payment captured |
| `payment.refunded` | Payment refunded |
| `payment.failed` | Payment failed |
| `credential.issued` | Verifiable Credential issued |
| `credential.revoked` | Verifiable Credential revoked |
| `ans.record.updated` | ANS record changed |
| `system.heartbeat` | Server heartbeat |
| `system.error` | Server-side error notification |

### 6.4 Subscription

After connection, the client sends a subscription message to indicate which event types it wants to receive:

```json
{
  "type": "subscribe",
  "events": [
    "payment.*",
    "did.created",
    "onboard.session.*"
  ]
}
```

**Server acknowledgment:**

```json
{
  "type": "subscribe_ack",
  "events": [
    "payment.*",
    "did.created",
    "onboard.session.*"
  ],
  "subscribedAt": "2026-03-30T12:00:01Z"
}
```

### 6.5 Wildcard Matching

Event type subscriptions support wildcard matching using `*`:

| Pattern | Matches |
|---|---|
| `payment.*` | `payment.captured`, `payment.refunded`, `payment.failed`, etc. |
| `onboard.session.*` | `onboard.session.initiated`, `onboard.session.provisioned`, etc. |
| `*` | All event types |
| `did.created` | Only `did.created` (exact match) |

Wildcards apply only to the final segment. `payment.*` matches `payment.captured` but not `payment.intent.created` (if such a multi-level type existed). For multi-level matching, subscribe to each level explicitly.

### 6.6 Unsubscription

```json
{
  "type": "unsubscribe",
  "events": ["payment.*"]
}
```

**Server acknowledgment:**

```json
{
  "type": "unsubscribe_ack",
  "events": ["payment.*"],
  "unsubscribedAt": "2026-03-30T12:10:00Z"
}
```

### 6.7 Heartbeat

The server sends a heartbeat event every 30 seconds to keep the connection alive and allow clients to detect stale connections:

```json
{
  "eventId": "evt_heartbeat_001",
  "type": "system.heartbeat",
  "timestamp": "2026-03-30T12:00:30Z",
  "source": "did:aicp:9XyZaBcDeFgHiJkLmNoPqR",
  "data": {
    "serverTime": "2026-03-30T12:00:30Z",
    "connectionAge": 30
  }
}
```

Clients SHOULD close and reconnect if no heartbeat is received within 90 seconds (3 missed heartbeats).

### 6.8 Client-to-Server Messages

Clients may send the following message types:

| Type | Description |
|---|---|
| `subscribe` | Subscribe to event types |
| `unsubscribe` | Unsubscribe from event types |
| `ping` | Client heartbeat (server responds with `pong`) |

Any other message type from the client is ignored with a `system.error` event.

---

## 7. Pagination

### 7.1 Cursor-Based Pagination

All list endpoints use cursor-based pagination for consistent results under concurrent modifications.

**Request parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `limit` | integer | 20 | Number of results per page. Min: 1, Max: 100. |
| `cursor` | string | (none) | Opaque cursor from a previous response. |

**Example request:**

```http
GET /aicp/v1/ans/search?q=inference&limit=10&cursor=eyJvZmZzZXQiOjEwfQ==
```

### 7.2 Response Format

```json
{
  "data": {
    "results": [
      { "...item 1..." },
      { "...item 2..." },
      { "...item N..." }
    ],
    "next_cursor": "eyJvZmZzZXQiOjIwfQ==",
    "has_more": true
  }
}
```

| Field | Type | Description |
|---|---|---|
| `results` | array | The page of results |
| `next_cursor` | string or null | Cursor for the next page. `null` if no more results. |
| `has_more` | boolean | `true` if more results exist beyond this page |

### 7.3 Cursor Format

Cursors are opaque base64url-encoded JSON objects. Clients MUST NOT parse or construct cursors --- they should only be passed back to the server as received. The internal structure is an implementation detail:

```json
// Internal (not for client use):
{"offset": 20, "sort": "relevance", "snapshot": "2026-03-30T12:00:00Z"}
```

### 7.4 Pagination Guarantees

- Results are stable within a cursor chain (no duplicates, no gaps) as long as the underlying data does not change.
- New items added after the first page request may or may not appear in subsequent pages.
- Deleted items will not cause gaps in subsequent pages.

---

## 8. Rate Limiting

### 8.1 Rate Limit Headers

All responses include rate limit headers:

```http
HTTP/1.1 200 OK
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 87
X-RateLimit-Reset: 1743339660
```

| Header | Type | Description |
|---|---|---|
| `X-RateLimit-Limit` | integer | Maximum requests allowed in the current window |
| `X-RateLimit-Remaining` | integer | Requests remaining in the current window |
| `X-RateLimit-Reset` | integer | Unix timestamp when the window resets |

### 8.2 Rate Limit Exceeded

When the rate limit is exceeded, the server responds with HTTP 429:

```http
HTTP/1.1 429 Too Many Requests
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1743339660
Retry-After: 42

{
  "error": {
    "code": "RATE_LIMITED",
    "message": "Too many requests",
    "detail": "Rate limit of 100 requests per minute exceeded. Retry after 42 seconds."
  }
}
```

The `Retry-After` header indicates the number of seconds to wait before retrying.

### 8.3 Rate Limit Tiers

| Endpoint Category | Limit | Window |
|---|---|---|
| DID resolution | 100 | 1 minute |
| ANS search | 30 | 1 minute |
| Onboarding | 10 | 1 minute |
| Payment operations | 60 | 1 minute |
| WebSocket ticket | 5 | 1 minute |
| General API | 100 | 1 minute |

Rate limits are applied per authenticated DID. Unauthenticated requests are rate-limited by IP address at a lower threshold (10 requests per minute).

---

## 9. Request Headers

### 9.1 Required Headers

| Header | Description |
|---|---|
| `Content-Type` | Must be `application/json` for request bodies |
| `Accept` | Should be `application/json` |

### 9.2 Authentication Headers

| Header | Description |
|---|---|
| `Authorization` | `DID <did> <signature>` or `Bearer <token>` |
| `X-AICP-Timestamp` | ISO 8601 timestamp, required for DID-Auth |

### 9.3 Optional Headers

| Header | Description |
|---|---|
| `X-Request-ID` | Client-generated request ID for tracing |
| `X-Idempotency-Key` | Idempotency key for write operations |
| `Cache-Control` | `no-cache` to bypass ANS record cache |

If `X-Request-ID` is provided, the server echoes it in the response. If not provided, the server generates one and includes it in the response as `X-Request-ID`.

---

## 10. Security Considerations

- **TLS requirement**: All HTTP and WebSocket connections MUST use TLS 1.2 or later. Plain HTTP/WS connections MUST be rejected.
- **Timestamp validation**: Servers MUST reject DID-Auth requests where `X-AICP-Timestamp` differs from the server clock by more than 5 minutes, to prevent replay attacks.
- **WebSocket ticket leakage**: WebSocket tickets are single-use and expire in 60 seconds. They MUST NOT be logged or stored beyond validation.
- **CORS**: Services SHOULD NOT set permissive CORS headers. Agent-to-service communication is server-to-server (not browser-based), so CORS is generally not applicable.
- **Request body limits**: Servers SHOULD enforce a maximum request body size of 1 MB to prevent denial-of-service via oversized payloads.
