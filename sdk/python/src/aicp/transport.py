"""HTTP transport for AICP API calls."""

from __future__ import annotations

from typing import Any, TypeVar

import httpx

from aicp.types import APIResponse

T = TypeVar("T")


class AICPError(Exception):
    """Error raised when an API request fails."""

    def __init__(
        self,
        status_code: int,
        code: str,
        message: str,
        detail: str | None = None,
    ) -> None:
        super().__init__(message)
        self.status_code = status_code
        self.code = code
        self.detail = detail


class Transport:
    """Async HTTP transport for AICP server communication."""

    def __init__(
        self,
        server_url: str,
        *,
        auth_token: str | None = None,
        timeout: float = 30.0,
        client: httpx.AsyncClient | None = None,
    ) -> None:
        self.base_url = server_url.rstrip("/")
        self._auth_token = auth_token
        self._timeout = timeout
        self._client = client or httpx.AsyncClient(timeout=timeout)
        self._owns_client = client is None

    def set_auth_token(self, token: str) -> None:
        """Update the auth token."""
        self._auth_token = token

    async def get(self, path: str) -> Any:
        """GET request."""
        return await self._request("GET", path)

    async def post(self, path: str, body: Any | None = None) -> Any:
        """POST request with JSON body."""
        return await self._request("POST", path, body)

    async def _request(self, method: str, path: str, body: Any | None = None) -> Any:
        url = f"{self.base_url}{path}"
        headers: dict[str, str] = {"Accept": "application/json"}
        if self._auth_token:
            headers["Authorization"] = f"Bearer {self._auth_token}"

        kwargs: dict[str, Any] = {"headers": headers}
        if body is not None:
            kwargs["json"] = body

        try:
            resp = await self._client.request(method, url, **kwargs)
        except httpx.TimeoutException as e:
            raise AICPError(0, "TIMEOUT", f"request timed out after {self._timeout}s") from e
        except httpx.HTTPError as e:
            raise AICPError(0, "NETWORK_ERROR", str(e)) from e

        data = resp.json()
        api_resp = APIResponse.model_validate(data)

        if not resp.is_success or not api_resp.success:
            if api_resp.error:
                raise AICPError(
                    resp.status_code,
                    api_resp.error.code,
                    api_resp.error.message,
                    api_resp.error.detail,
                )
            raise AICPError(resp.status_code, "UNKNOWN_ERROR", f"HTTP {resp.status_code}")

        return api_resp.data

    async def close(self) -> None:
        """Close the underlying HTTP client."""
        if self._owns_client:
            await self._client.aclose()

    async def __aenter__(self) -> Transport:
        return self

    async def __aexit__(self, *args: Any) -> None:
        await self.close()
