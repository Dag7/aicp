---
sidebar_position: 1
slug: /intro
title: Introduction
---

# Agentic Identity and Commerce Protocol

AICP is an open-source protocol that gives autonomous AI agents the foundational
capabilities they need to operate in the real world: verifiable identity,
service discovery, structured onboarding, and provider-agnostic payments.

## The Problem

As AI agents become more autonomous, they need to interact with services, other
agents, and payment systems without constant human intervention. Today there is
no standard way for an agent to:

- Prove who it is and who authorized it
- Find and evaluate services programmatically
- Register with a new service in a machine-readable flow
- Pay for resources across different payment providers

## The Four Layers

AICP addresses these gaps through a layered protocol stack:

| Layer | Name         | Purpose                                           |
| ----- | ------------ | ------------------------------------------------- |
| 1     | **Identity**   | W3C DID-based cryptographic identity anchored to human or organization principals |
| 2     | **Discovery**  | Agent Name System (ANS) for service lookup and capability advertisement |
| 3     | **Onboarding** | Standardized four-phase registration flow (init, challenge, verify, activate) |
| 4     | **Commerce**   | Provider-agnostic payment abstraction supporting Stripe, PayPal, and USDC |

Each layer builds on the one below it. An agent first establishes identity,
then uses that identity to discover services, onboard with them, and finally
transact.

## Key Design Decisions

- **DID format**: `did:aicp:<principal-id>` for principals, `did:aicp:<principal-id>:<agent-id>` for agents
- **Cryptography**: Ed25519 key pairs, JWS-signed tokens, Merkle-tree anchored ledger
- **Storage**: PostgreSQL-backed consortium ledger with append-only blocks
- **Transport**: REST over HTTPS with optional WebSocket for real-time events

## Next Steps

Ready to dive in? Head to the [Quickstart](./quickstart.md) to have an agent
making its first payment in five API calls.
