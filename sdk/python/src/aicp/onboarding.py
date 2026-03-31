"""Onboarding flow orchestrator."""

from __future__ import annotations

from typing import Any

from aicp.crypto import create_proof
from aicp.transport import Transport
from aicp.types import (
    AuthorizeResponse,
    DIDAuthResponse,
    NegotiateResponse,
    OnboardInitResponse,
    OnboardingState,
    ProvisionResponse,
    VerifiablePresentation,
)


class OnboardingManager:
    """Orchestrates the 4-phase onboarding flow."""

    def __init__(self, transport: Transport) -> None:
        self._transport = transport

    async def initiate(
        self,
        agent_did: str,
        presentation: VerifiablePresentation,
        requested_capabilities: list[str],
    ) -> OnboardInitResponse:
        """Phase 1: Initiate onboarding."""
        data = await self._transport.post(
            "/aicp/v1/onboard",
            {
                "agent_did": agent_did,
                "presentation": presentation.model_dump(by_alias=True),
                "requested_capabilities": requested_capabilities,
            },
        )
        return OnboardInitResponse.model_validate(data)

    async def negotiate(
        self,
        session_id: str,
        offered_capabilities: list[dict[str, str]],
        challenge_response: DIDAuthResponse | None = None,
    ) -> NegotiateResponse:
        """Phase 2: Negotiate capabilities."""
        body: dict[str, Any] = {
            "session_id": session_id,
            "offered_capabilities": offered_capabilities,
        }
        if challenge_response is not None:
            body["challenge_response"] = challenge_response.model_dump(by_alias=True)
        data = await self._transport.post(
            f"/aicp/v1/onboard/{session_id}/negotiate", body
        )
        return NegotiateResponse.model_validate(data)

    async def authorize(
        self,
        session_id: str,
        payment_token: str,
    ) -> AuthorizeResponse:
        """Phase 3: Authorize payment."""
        data = await self._transport.post(
            f"/aicp/v1/onboard/{session_id}/authorize",
            {"session_id": session_id, "payment_token": payment_token},
        )
        return AuthorizeResponse.model_validate(data)

    async def provision(self, session_id: str) -> ProvisionResponse:
        """Phase 4: Provision the agent."""
        data = await self._transport.post(
            f"/aicp/v1/onboard/{session_id}/provision",
            {"session_id": session_id},
        )
        return ProvisionResponse.model_validate(data)

    async def run_full_flow(
        self,
        agent_did: str,
        presentation: VerifiablePresentation,
        requested_capabilities: list[str],
        private_key: bytes,
        payment_token: str,
    ) -> dict[str, Any]:
        """Run the full 4-phase onboarding flow."""
        # Phase 1
        init_resp = await self.initiate(agent_did, presentation, requested_capabilities)

        # Build challenge response if needed.
        challenge_response: DIDAuthResponse | None = None
        if init_resp.challenge:
            proof = create_proof(
                {"challenge": init_resp.challenge.challenge, "did": agent_did},
                private_key,
                f"{agent_did}#key-1",
                "authentication",
                challenge=init_resp.challenge.challenge,
                domain=init_resp.challenge.domain,
            )
            challenge_response = DIDAuthResponse(
                type="DIDAuthResponse",
                challenge=init_resp.challenge.challenge,
                did=agent_did,
                proof=proof,
            )

        # Phase 2
        await self.negotiate(
            init_resp.session_id,
            [{"name": c} for c in requested_capabilities],
            challenge_response,
        )

        # Phase 3
        await self.authorize(init_resp.session_id, payment_token)

        # Phase 4
        provision_resp = await self.provision(init_resp.session_id)

        return {
            "state": provision_resp.state,
            "account_id": provision_resp.account_id,
            "credentials": provision_resp.credentials,
        }
