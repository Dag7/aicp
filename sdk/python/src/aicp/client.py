"""Main AICP agent client — the 5-call experience."""

from __future__ import annotations

from typing import Any

from aicp.commerce import CommerceManager
from aicp.crypto import (
    KeyPair,
    did_identifier,
    generate_key_pair,
    get_public_key,
    principal_did,
    sign_json,
)
from aicp.discovery import DiscoveryManager
from aicp.identity import IdentityManager
from aicp.onboarding import OnboardingManager
from aicp.transport import Transport
from aicp.types import (
    ANSRecord,
    MoneyAmount,
    PaymentExecuteResponse,
    VerifiableCredential,
    VerifiablePresentation,
    VerifyResponse,
)
from aicp.wallet import CredentialWallet, MemoryWalletStorage, WalletStorage


class AICPAgent:
    """Primary entry point for the Python SDK.

    Provides a high-level, 5-call experience for agent developers:
      1. discover(domain_or_did) — find a service
      2. onboard(service, capabilities, token) — register with a service
      3. pay(service, amount) — make a payment
      4. verify(did) — verify an identity
      5. health() — check server health

    Example::

        agent = await AICPAgent.create(
            server_url="https://aicp.example.com",
            private_key=my_private_key,
        )
        service = await agent.discover("example.com")
        result = await agent.verify("did:aicp:abc123")
    """

    def __init__(
        self,
        transport: Transport,
        private_key: bytes,
        wallet: CredentialWallet,
        *,
        agent_did_override: str | None = None,
        principal_did_str: str | None = None,
    ) -> None:
        self._transport = transport
        self._private_key = private_key
        self._public_key = get_public_key(private_key)
        self._wallet = wallet

        self.identity = IdentityManager(transport)
        self.discovery = DiscoveryManager(transport)
        self.onboarding = OnboardingManager(transport)
        self.commerce = CommerceManager(transport)

        if agent_did_override:
            self._did = agent_did_override
        elif principal_did_str:
            self._did = f"{principal_did_str}:{did_identifier(self._public_key)}"
        else:
            self._did = principal_did(self._public_key)

    @classmethod
    async def create(
        cls,
        server_url: str,
        private_key: bytes,
        *,
        agent_did: str | None = None,
        principal_did_str: str | None = None,
        auth_token: str | None = None,
        storage: WalletStorage | None = None,
    ) -> AICPAgent:
        """Create a new AICPAgent."""
        transport = Transport(server_url, auth_token=auth_token)
        wallet = CredentialWallet(storage or MemoryWalletStorage())
        agent = cls(
            transport,
            private_key,
            wallet,
            agent_did_override=agent_did,
            principal_did_str=principal_did_str,
        )
        await wallet.store_key_pair(
            agent.did,
            KeyPair(public_key=agent._public_key, private_key=private_key),
        )
        return agent

    @classmethod
    async def generate_and_connect(
        cls,
        server_url: str,
        storage: WalletStorage | None = None,
    ) -> AICPAgent:
        """Generate a new identity and connect."""
        kp = generate_key_pair()
        return await cls.create(server_url, kp.private_key, storage=storage)

    @property
    def did(self) -> str:
        """Get this agent's DID."""
        return self._did

    @property
    def public_key(self) -> bytes:
        """Get this agent's public key."""
        return self._public_key

    @property
    def wallet(self) -> CredentialWallet:
        """Get the credential wallet."""
        return self._wallet

    # ─── High-Level API ─────────────────────────────────────────────────

    async def discover(self, service_identifier: str) -> ANSRecord:
        """1. Discover a service by domain or DID."""
        return await self.discovery.resolve(service_identifier)

    async def onboard(
        self,
        service: ANSRecord,
        requested_capabilities: list[str],
        payment_token: str,
        credentials: list[VerifiableCredential] | None = None,
    ) -> dict[str, Any]:
        """2. Onboard to a service (full 4-phase flow)."""
        presentation = self.identity.build_presentation(
            self._did, credentials or [], self._private_key
        )
        return await self.onboarding.run_full_flow(
            agent_did=self._did,
            presentation=presentation,
            requested_capabilities=requested_capabilities,
            private_key=self._private_key,
            payment_token=payment_token,
        )

    async def pay(
        self,
        service: ANSRecord,
        amount: MoneyAmount,
        purpose: str = "usage-payment",
    ) -> PaymentExecuteResponse:
        """3. Make a payment."""
        return await self.commerce.pay(
            agent_did=self._did,
            service_did=service.service.did,
            amount=amount,
            purpose=purpose,
            service_category=service.service.category[0] if service.service.category else "general",
        )

    async def verify(self, did: str) -> VerifyResponse:
        """4. Verify a DID."""
        return await self.identity.verify(did)

    async def health(self) -> bool:
        """5. Check server health."""
        try:
            await self._transport.get("/health")
            return True
        except Exception:
            return False

    # ─── Utility ────────────────────────────────────────────────────────

    def sign(self, data: Any) -> str:
        """Create a signed proof over arbitrary data."""
        return sign_json(data, self._private_key)

    def build_presentation(
        self, credentials: list[VerifiableCredential]
    ) -> VerifiablePresentation:
        """Build and sign a verifiable presentation."""
        return self.identity.build_presentation(
            self._did, credentials, self._private_key
        )

    async def close(self) -> None:
        """Close the transport."""
        await self._transport.close()

    async def __aenter__(self) -> AICPAgent:
        return self

    async def __aexit__(self, *args: Any) -> None:
        await self.close()


class AICPClient:
    """Simple API client (no identity) for read-only operations."""

    def __init__(self, server_url: str, *, auth_token: str | None = None) -> None:
        self._transport = Transport(server_url, auth_token=auth_token)
        self.identity = IdentityManager(self._transport)
        self.discovery = DiscoveryManager(self._transport)
        self.commerce = CommerceManager(self._transport)

    async def verify(self, did: str) -> VerifyResponse:
        """Verify a DID."""
        return await self.identity.verify(did)

    async def discover(self, service_identifier: str) -> ANSRecord:
        """Discover a service."""
        return await self.discovery.resolve(service_identifier)

    async def health(self) -> bool:
        """Check server health."""
        try:
            await self._transport.get("/health")
            return True
        except Exception:
            return False

    async def close(self) -> None:
        """Close the transport."""
        await self._transport.close()

    async def __aenter__(self) -> AICPClient:
        return self

    async def __aexit__(self, *args: Any) -> None:
        await self.close()
