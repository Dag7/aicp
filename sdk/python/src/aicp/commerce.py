"""Commerce operations (payment tokens, execution, status)."""

from __future__ import annotations

from urllib.parse import quote

from aicp.transport import Transport
from aicp.types import (
    MoneyAmount,
    PaymentExecuteResponse,
    PaymentStatusResponse,
    PaymentTokenResponse,
)


class CommerceManager:
    """Manages payment operations."""

    def __init__(self, transport: Transport) -> None:
        self._transport = transport

    async def request_token(
        self,
        agent_did: str,
        service_did: str,
        amount: MoneyAmount,
        purpose: str,
        service_category: str,
        max_amount: MoneyAmount | None = None,
    ) -> PaymentTokenResponse:
        """Request a payment token."""
        body = {
            "agent_did": agent_did,
            "service_did": service_did,
            "amount": amount.model_dump(),
            "purpose": purpose,
            "service_category": service_category,
        }
        if max_amount is not None:
            body["max_amount"] = max_amount.model_dump()
        data = await self._transport.post("/aicp/v1/payment/token", body)
        return PaymentTokenResponse.model_validate(data)

    async def execute(self, token: str) -> PaymentExecuteResponse:
        """Execute a payment with a token."""
        data = await self._transport.post("/aicp/v1/payment/execute", {"token": token})
        return PaymentExecuteResponse.model_validate(data)

    async def get_status(self, payment_id: str) -> PaymentStatusResponse:
        """Check payment status."""
        data = await self._transport.get(
            f"/aicp/v1/payment/status/{quote(payment_id, safe='')}"
        )
        return PaymentStatusResponse.model_validate(data)

    async def pay(
        self,
        agent_did: str,
        service_did: str,
        amount: MoneyAmount,
        purpose: str,
        service_category: str,
    ) -> PaymentExecuteResponse:
        """Request a token and immediately execute the payment."""
        token_resp = await self.request_token(
            agent_did, service_did, amount, purpose, service_category
        )
        return await self.execute(token_resp.token)
