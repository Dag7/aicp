# AICP Error Codes Specification

**Version:** 1.0.0
**Status:** Draft
**Last Updated:** 2026-03-30

---

## 1. Overview

This document defines all error codes used in the AICP protocol, their mapping to HTTP status codes, the standard error response format, and guidance on error handling for client implementations.

---

## 2. Error Code Table

| Code | HTTP Status | Description |
|---|---|---|
| `NOT_FOUND` | 404 | The requested resource was not found |
| `BAD_REQUEST` | 400 | The request is malformed or invalid |
| `UNAUTHORIZED` | 401 | Authentication is required |
| `FORBIDDEN` | 403 | You do not have permission |
| `CONFLICT` | 409 | The resource already exists |
| `RATE_LIMITED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | An internal error occurred |
| `DID_NOT_FOUND` | 404 | The specified DID was not found |
| `DID_DEACTIVATED` | 410 | The specified DID has been deactivated |
| `CREDENTIAL_REVOKED` | 403 | The presented credential has been revoked |
| `CREDENTIAL_EXPIRED` | 403 | The presented credential has expired |
| `PAYMENT_TOKEN_EXPIRED` | 403 | The payment token has expired |
| `PAYMENT_TOKEN_REPLAY` | 409 | The payment token has already been used |
| `SPENDING_LIMIT_EXCEEDED` | 403 | The transaction exceeds the agent's spending limit |
| `INVALID_PHASE` | 400 | Invalid onboarding phase transition |
| `SESSION_EXPIRED` | 410 | The onboarding session has expired |
| `PAYMENT_FAILED` | 402 | Payment processing failed |

---

## 3. Error Response Format

### 3.1 Structure

All error responses use the following JSON envelope:

```json
{
  "error": {
    "code": "CREDENTIAL_REVOKED",
    "message": "The presented credential has been revoked",
    "detail": "Credential urn:uuid:3e671c42-a4f8-4b90-b5d3-8c1a4e7f2d9b was revoked at 2026-03-29T18:00:00Z"
  }
}
```

### 3.2 Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `code` | string | Yes | Machine-readable error code from the table above. Clients MUST switch on this field, not on the HTTP status code or message. |
| `message` | string | Yes | Human-readable description. Suitable for logging but not for programmatic logic. Messages may change between protocol versions. |
| `detail` | string | No | Additional context specific to the error instance. May include resource identifiers, timestamps, or constraint details. |

### 3.3 Examples

**DID not found:**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "error": {
    "code": "DID_NOT_FOUND",
    "message": "The specified DID was not found",
    "detail": "did:aicp:7Hk3dQwPvLmNxR9sTuVwXy could not be resolved in the registry"
  }
}
```

**Spending limit exceeded:**

```http
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "error": {
    "code": "SPENDING_LIMIT_EXCEEDED",
    "message": "The transaction exceeds the agent's spending limit",
    "detail": "Daily limit is 500.00 USD; current daily spend is 487.50 USD; requested amount is 25.00 USD"
  }
}
```

**Payment token replay:**

```http
HTTP/1.1 409 Conflict
Content-Type: application/json

{
  "error": {
    "code": "PAYMENT_TOKEN_REPLAY",
    "message": "The payment token has already been used",
    "detail": "Token jti pt_a1b2c3d4-e5f6-7890-abcd-ef1234567890 was consumed at 2026-03-30T12:05:00Z"
  }
}
```

**Invalid onboarding phase:**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": {
    "code": "INVALID_PHASE",
    "message": "Invalid onboarding phase transition",
    "detail": "Cannot transition from 'initiated' to 'payment_authorized'; expected 'identity_presented'"
  }
}
```

**Rate limited:**

```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/json
Retry-After: 42
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1743339660

{
  "error": {
    "code": "RATE_LIMITED",
    "message": "Too many requests",
    "detail": "Rate limit of 100 requests per minute exceeded. Retry after 42 seconds."
  }
}
```

---

## 4. Error Handling Best Practices

### 4.1 General Guidance

1. **Switch on `code`, not HTTP status.** Multiple error codes may map to the same HTTP status (e.g., `CREDENTIAL_REVOKED`, `CREDENTIAL_EXPIRED`, and `SPENDING_LIMIT_EXCEEDED` all return 403). The `code` field provides the precise error type.

2. **Do not parse `message` or `detail` programmatically.** These fields are for human consumption and logging. Their exact text may change between server versions.

3. **Log the full error response.** Always log `code`, `message`, and `detail` together with the request context for debugging.

4. **Include `X-Request-ID` in requests.** This allows correlating client-side errors with server-side logs.

### 4.2 Authentication Errors

When receiving `UNAUTHORIZED` (401):

1. If using DID-Auth, verify the timestamp is fresh (within 5 minutes).
2. Verify the canonical request representation matches what the server expects.
3. Verify the signing key is listed in the DID Document's `authentication` relationship.
4. If the key was recently rotated, ensure the new key's DID Document has propagated.

When receiving `FORBIDDEN` (403):

1. Check the specific `code` to determine the cause:
   - `CREDENTIAL_REVOKED`: The credential was revoked by its issuer. Obtain a new credential from the principal.
   - `CREDENTIAL_EXPIRED`: The credential's `expirationDate` has passed. Request a renewed credential from the principal.
   - `PAYMENT_TOKEN_EXPIRED`: Generate a new payment token with a fresh `iat`/`exp`.
   - `SPENDING_LIMIT_EXCEEDED`: Wait for the daily/monthly window to reset, or request increased limits from the principal.

### 4.3 Onboarding Errors

When receiving `INVALID_PHASE` (400):

1. Check the `detail` field to understand the expected phase transition.
2. Query the session status to determine the current phase.
3. Restart onboarding if the session state is irrecoverable.

When receiving `SESSION_EXPIRED` (410):

1. Start a new onboarding session from the `initiated` phase.
2. All previous session state is lost.

### 4.4 Payment Errors

When receiving `PAYMENT_TOKEN_REPLAY` (409):

1. If retrying a failed request, check if the original request actually succeeded (the token was consumed).
2. Use the `idempotencyKey` mechanism for safe retries instead of resubmitting the same token.

When receiving `PAYMENT_FAILED` (402):

1. Check the `detail` for the underlying payment provider error.
2. The payment intent may need to be recreated with a different payment method or adapter.

---

## 5. Retry Semantics

### 5.1 Retryable Errors

The following errors are safe to retry with the same request:

| Code | Retry Strategy |
|---|---|
| `RATE_LIMITED` | Wait for `Retry-After` seconds, then retry. Use jitter. |
| `INTERNAL_ERROR` | Retry with exponential backoff: `min(30s, 2^attempt * 100ms)` + jitter. Maximum 3 retries. |
| `PAYMENT_FAILED` | Retry once after 5 seconds. If it fails again, do not retry automatically. |

### 5.2 Non-Retryable Errors

The following errors should NOT be retried with the same request:

| Code | Reason |
|---|---|
| `NOT_FOUND` | The resource does not exist. Retrying will not create it. |
| `BAD_REQUEST` | The request is malformed. Fix the request before retrying. |
| `UNAUTHORIZED` | Authentication is invalid. Reauthenticate before retrying. |
| `FORBIDDEN` | Permission denied. Resolve the permission issue first. |
| `CONFLICT` | Duplicate resource. The operation already succeeded or conflicts with existing state. |
| `DID_NOT_FOUND` | The DID does not exist. Verify the DID is correct. |
| `DID_DEACTIVATED` | The DID has been permanently deactivated. |
| `CREDENTIAL_REVOKED` | The credential has been permanently revoked. |
| `CREDENTIAL_EXPIRED` | The credential has expired. Obtain a new credential. |
| `PAYMENT_TOKEN_EXPIRED` | The token has expired. Generate a new one. |
| `PAYMENT_TOKEN_REPLAY` | The token was already consumed. This may indicate the original request succeeded. |
| `SPENDING_LIMIT_EXCEEDED` | Spending limits cannot be resolved by retrying. |
| `INVALID_PHASE` | The state machine does not allow this transition. |
| `SESSION_EXPIRED` | The session is gone. Start a new session. |

### 5.3 Exponential Backoff

For retryable errors, clients MUST implement exponential backoff with jitter:

```
delay = min(max_delay, base_delay * 2^attempt) + random(0, jitter)
```

Recommended values:

| Parameter | Value |
|---|---|
| `base_delay` | 100ms |
| `max_delay` | 30s |
| `jitter` | 0 to 100ms (uniform random) |
| `max_attempts` | 3 |

### 5.4 Idempotency

For payment operations, always use the `idempotencyKey` claim in the payment token. This ensures that retried requests return the result of the original operation rather than executing the payment twice.

---

## 6. WebSocket Error Events

### 6.1 Error Event Format

Errors occurring on a WebSocket connection are delivered as `system.error` events:

```json
{
  "eventId": "evt_err_a1b2c3d4",
  "type": "system.error",
  "timestamp": "2026-03-30T12:05:00Z",
  "source": "did:aicp:9XyZaBcDeFgHiJkLmNoPqR",
  "data": {
    "code": "RATE_LIMITED",
    "message": "WebSocket event rate limit exceeded",
    "detail": "Maximum 100 subscriptions per connection",
    "severity": "warning"
  }
}
```

### 6.2 Severity Levels

| Severity | Description | Client Action |
|---|---|---|
| `info` | Informational message, no action needed | Log for debugging |
| `warning` | Degraded behavior, connection remains open | Log and adjust behavior |
| `error` | Operation failed, connection remains open | Log, retry the operation if applicable |
| `fatal` | Connection will be closed by the server | Log, reconnect with backoff |

### 6.3 Connection Closure Codes

When the server closes a WebSocket connection, it uses standard WebSocket close codes:

| Code | Meaning | AICP Reason |
|---|---|---|
| 1000 | Normal Closure | Server shutting down gracefully |
| 1001 | Going Away | Server restarting |
| 1008 | Policy Violation | Authentication failure or DID deactivated |
| 1011 | Internal Error | Unexpected server error |
| 4000 | Ticket Expired | WebSocket ticket was expired or invalid |
| 4001 | Ticket Already Used | WebSocket ticket was already consumed |
| 4002 | Rate Limited | Too many connections from this DID |
| 4003 | Session Revoked | The agent's session or credentials were revoked |

Codes 4000-4003 are application-defined codes in the private use range (4000-4999).

### 6.4 Reconnection

After an unexpected disconnection, clients SHOULD reconnect using exponential backoff:

1. Wait `min(30s, 1s * 2^attempt)` + random jitter.
2. Obtain a new WebSocket ticket via `POST /aicp/v1/ws/ticket`.
3. Reconnect and resubscribe to the desired event types.
4. Maximum 5 reconnection attempts before alerting the operator.

---

## 7. Error Code Categories

For convenience, error codes can be grouped into categories:

### 7.1 General Errors

| Code | Description |
|---|---|
| `NOT_FOUND` | Generic resource not found |
| `BAD_REQUEST` | Generic malformed request |
| `UNAUTHORIZED` | Generic authentication failure |
| `FORBIDDEN` | Generic permission denial |
| `CONFLICT` | Generic conflict |
| `RATE_LIMITED` | Rate limit exceeded |
| `INTERNAL_ERROR` | Unrecoverable server error |

### 7.2 Identity Errors

| Code | Description |
|---|---|
| `DID_NOT_FOUND` | DID resolution failed |
| `DID_DEACTIVATED` | DID has been deactivated |
| `CREDENTIAL_REVOKED` | Verifiable Credential revoked |
| `CREDENTIAL_EXPIRED` | Verifiable Credential expired |

### 7.3 Commerce Errors

| Code | Description |
|---|---|
| `PAYMENT_TOKEN_EXPIRED` | Payment token past expiration |
| `PAYMENT_TOKEN_REPLAY` | Duplicate payment token |
| `SPENDING_LIMIT_EXCEEDED` | Agent spending limit breached |
| `PAYMENT_FAILED` | Payment adapter returned an error |

### 7.4 Onboarding Errors

| Code | Description |
|---|---|
| `INVALID_PHASE` | Invalid state machine transition |
| `SESSION_EXPIRED` | Onboarding session timed out |
