---
sidebar_position: 2
title: Quickstart
---

# Five-Call Agent Quickstart

This guide walks you through the complete AICP flow -- from installing the SDK
to making a payment -- in five steps.

## Prerequisites

- Node.js 18+
- A running AICP server (see [Server Installation](./server/installation.md))

## 1. Install the SDK

```bash
npm install @aicp/sdk
```

## 2. Create an Identity

Every agent needs a DID (Decentralized Identifier) anchored to a principal.
The SDK generates an Ed25519 key pair and registers the identity on the
consortium ledger.

```typescript
import { AICPClient } from "@aicp/sdk";

const client = new AICPClient({ serverUrl: "https://api.aicp.dev" });

// Create a principal identity (represents the human or org)
const principal = await client.identity.createPrincipal({
  name: "Acme Corp",
});
console.log(principal.did); // did:aicp:7Xk9bQ...

// Create an agent identity under that principal
const agent = await client.identity.createAgent({
  principalDid: principal.did,
  name: "purchasing-bot",
  capabilities: ["commerce:pay", "discovery:resolve"],
});
console.log(agent.did); // did:aicp:7Xk9bQ...:3nRtYp...
```

## 3. Discover a Service

Use the Agent Name System to find a service by name or capability.

```typescript
const results = await client.discovery.resolve("compute.providers.aicp.dev");

console.log(results.endpoint); // https://compute.example.com
console.log(results.capabilities); // ["vm:create", "vm:destroy", ...]
console.log(results.paymentMethods); // ["stripe", "usdc"]
```

## 4. Onboard with the Service

The four-phase onboarding flow registers your agent with the discovered service.
The server handles the challenge-response exchange automatically.

```typescript
const session = await client.onboarding.start({
  agentDid: agent.did,
  serviceEndpoint: results.endpoint,
});

// The SDK drives the state machine: init -> challenge -> verify -> active
const registration = await client.onboarding.complete(session.id);

console.log(registration.status); // "active"
console.log(registration.credentials); // Service-issued credentials
```

## 5. Make a Payment

With an active registration, your agent can now pay for resources using
whichever payment provider the service supports.

```typescript
const payment = await client.commerce.pay({
  agentDid: agent.did,
  recipientDid: results.providerDid,
  amount: "25.00",
  currency: "USD",
  adapter: "stripe", // or "paypal", "usdc"
  memo: "GPU hours - 2025-03-31",
});

console.log(payment.receiptId); // Ledger-anchored receipt
console.log(payment.status); // "settled"
```

## What Just Happened?

In five calls your agent:

1. **Established identity** -- created a cryptographically verifiable DID
2. **Discovered a service** -- resolved an ANS name to an endpoint and its capabilities
3. **Onboarded** -- completed a four-phase registration with challenge-response verification
4. **Paid** -- executed a provider-agnostic payment and received a ledger-anchored receipt

Every step produced artifacts (DID documents, credentials, receipts) that are
anchored in the consortium ledger and independently verifiable.

## Next Steps

- [Architecture Overview](./architecture.md) -- understand the protocol layers in depth
- [Identity Layer](./protocol/identity.md) -- DIDs, delegation, and revocation
- [Server Configuration](./server/configuration.md) -- tune your AICP server
- [TypeScript SDK](./sdks/typescript.md) -- full SDK reference
