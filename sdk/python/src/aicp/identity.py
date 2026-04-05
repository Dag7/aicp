"""Identity management for AICP agents."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from aicp.crypto import (
    KeyPair,
    agent_did,
    create_proof,
    generate_key_pair,
    principal_did,
)
from aicp.types import (
    Proof,
    VerifiableCredential,
    VerifiablePresentation,
    VerifyResponse,
)

if TYPE_CHECKING:
    from aicp.transport import Transport


class IdentityManager:
    """Manages DID identity operations."""

    def __init__(self, transport: Transport) -> None:
        self._transport = transport

    def generate_principal(self) -> tuple[KeyPair, str]:
        """Generate a new principal identity (key pair + DID)."""
        kp = generate_key_pair()
        return kp, principal_did(kp.public_key)

    def generate_agent(self, principal_kp: KeyPair) -> tuple[KeyPair, str, str]:
        """Generate a new agent identity under a principal.

        Returns (kp, agent_did, principal_did).
        """
        agent_kp = generate_key_pair()
        p_did = principal_did(principal_kp.public_key)
        a_did = agent_did(principal_kp.public_key, agent_kp.public_key)
        return agent_kp, a_did, p_did

    async def verify(
        self,
        did: str,
        presentation: VerifiablePresentation | None = None,
    ) -> VerifyResponse:
        """Verify a DID and optionally a verifiable presentation."""
        body: dict[str, Any] = {"did": did}
        if presentation is not None:
            body["presentation"] = presentation.model_dump(by_alias=True)
        data = await self._transport.post("/aicp/v1/verify", body)
        return VerifyResponse.model_validate(data)

    def create_did_auth_proof(
        self,
        private_key: bytes,
        did: str,
        challenge: str,
        domain: str,
    ) -> Proof:
        """Create a signed DID-Auth proof."""
        return create_proof(
            {"challenge": challenge, "did": did},
            private_key,
            f"{did}#key-1",
            "authentication",
            challenge=challenge,
            domain=domain,
        )

    def build_presentation(
        self,
        holder_did: str,
        credentials: list[VerifiableCredential],
        private_key: bytes,
    ) -> VerifiablePresentation:
        """Build a signed verifiable presentation."""
        presentation_data = {
            "@context": [
                "https://www.w3.org/2018/credentials/v1",
                "https://aicp.dev/contexts/v1",
            ],
            "type": ["VerifiablePresentation"],
            "holder": holder_did,
            "verifiableCredential": [
                c.model_dump(by_alias=True) for c in credentials
            ],
        }

        proof = create_proof(
            presentation_data,
            private_key,
            f"{holder_did}#key-1",
            "authentication",
        )

        return VerifiablePresentation(
            **{
                "@context": presentation_data["@context"],
                "type": presentation_data["type"],
                "holder": holder_did,
                "verifiableCredential": credentials,
                "proof": proof,
            }
        )
