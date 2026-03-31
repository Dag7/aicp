"""Service provider integration helpers."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Callable

from aicp.types import ANSRecord, ANSServiceInfo, ANSOnboardingInfo, ANSAuthInfo


@dataclass
class ServiceProviderConfig:
    """Configuration for an AICP service provider."""

    service_did: str
    service_name: str
    categories: list[str]
    onboarding_flows: list[str] = field(default_factory=lambda: ["standard"])
    required_credentials: list[str] = field(
        default_factory=lambda: ["AICPAgentDelegation"]
    )
    accepted_payments: list[str] = field(default_factory=lambda: ["stripe"])
    minimum_trust_score: int = 100


@dataclass
class AICPRequestContext:
    """Request context with AICP auth info."""

    did: str | None = None
    authenticated: bool = False
    token: str | None = None


class AICPServiceAdapter:
    """Helps service providers integrate with the AICP protocol."""

    def __init__(self, config: ServiceProviderConfig) -> None:
        self._config = config

    def build_ans_record(
        self,
        *,
        description: str | None = None,
        homepage: str | None = None,
        api_base: str | None = None,
        pricing_model: str | None = None,
        pricing_tiers: list[dict[str, str]] | None = None,
    ) -> ANSRecord:
        """Generate the ANS record for .well-known/aicp.json."""
        pricing = None
        if pricing_model:
            from aicp.types import ANSPricingInfo, ANSPricingTier

            tiers = (
                [ANSPricingTier(**t) for t in pricing_tiers] if pricing_tiers else None
            )
            pricing = ANSPricingInfo(model=pricing_model, tiers=tiers)

        endpoints = None
        if api_base:
            from aicp.types import ANSEndpoints

            endpoints = ANSEndpoints(api_base=api_base)

        return ANSRecord(
            aicp_version="1.0",
            service=ANSServiceInfo(
                name=self._config.service_name,
                did=self._config.service_did,
                description=description,
                category=self._config.categories,
                homepage=homepage,
            ),
            onboarding=ANSOnboardingInfo(
                flows=self._config.onboarding_flows,
                required_credentials=self._config.required_credentials,
                accepted_payments=self._config.accepted_payments,
                minimum_trust_score=self._config.minimum_trust_score,
            ),
            authentication=ANSAuthInfo(
                did_methods=["did:aicp"],
                auth_protocols=["DIDAuth"],
            ),
            pricing=pricing,
            endpoints=endpoints,
        )

    def extract_auth(self, auth_header: str | None = None) -> AICPRequestContext:
        """Extract AICP auth context from an Authorization header."""
        if not auth_header or not auth_header.startswith("Bearer "):
            return AICPRequestContext(authenticated=False)

        token = auth_header[7:]
        return AICPRequestContext(did=token, authenticated=True, token=token)

    def require_auth(self, auth_header: str | None = None) -> AICPRequestContext:
        """Extract auth and raise if unauthenticated."""
        ctx = self.extract_auth(auth_header)
        if not ctx.authenticated:
            raise PermissionError("AICP authentication required")
        return ctx

    @property
    def service_did(self) -> str:
        """Get the service DID."""
        return self._config.service_did
