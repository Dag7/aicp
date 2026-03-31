"""Service discovery via ANS (Agent Name System)."""

from __future__ import annotations

import httpx

from aicp.transport import Transport
from aicp.types import ANSRecord


class DiscoveryManager:
    """Manages service discovery via ANS."""

    def __init__(self, transport: Transport) -> None:
        self._transport = transport

    async def fetch_well_known(self, domain: str) -> ANSRecord:
        """Fetch .well-known/aicp.json from a domain."""
        url = f"https://{domain}/.well-known/aicp.json"
        async with httpx.AsyncClient() as client:
            resp = await client.get(url, headers={"Accept": "application/json"})
            resp.raise_for_status()
            return ANSRecord.model_validate(resp.json())

    async def get_server_info(self) -> ANSRecord:
        """Fetch the server's own .well-known/aicp.json."""
        data = await self._transport.get("/.well-known/aicp.json")
        return ANSRecord.model_validate(data)

    async def resolve(self, service_identifier: str) -> ANSRecord:
        """Resolve a service by DID or domain."""
        if not service_identifier.startswith("did:"):
            try:
                return await self.fetch_well_known(service_identifier)
            except Exception:
                pass
        return await self.get_server_info()
