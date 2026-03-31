"""Credential wallet with pluggable storage."""

from __future__ import annotations

import json
import os
from abc import ABC, abstractmethod
from pathlib import Path
from urllib.parse import quote, unquote

from aicp.crypto import KeyPair
from aicp.types import VerifiableCredential


class WalletStorage(ABC):
    """Storage backend interface for the credential wallet."""

    @abstractmethod
    async def get(self, key: str) -> str | None: ...

    @abstractmethod
    async def set(self, key: str, value: str) -> None: ...

    @abstractmethod
    async def delete(self, key: str) -> None: ...

    @abstractmethod
    async def list_keys(self, prefix: str) -> list[str]: ...


class MemoryWalletStorage(WalletStorage):
    """In-memory wallet storage (for testing or ephemeral agents)."""

    def __init__(self) -> None:
        self._store: dict[str, str] = {}

    async def get(self, key: str) -> str | None:
        return self._store.get(key)

    async def set(self, key: str, value: str) -> None:
        self._store[key] = value

    async def delete(self, key: str) -> None:
        self._store.pop(key, None)

    async def list_keys(self, prefix: str) -> list[str]:
        return [k for k in self._store if k.startswith(prefix)]


class FileWalletStorage(WalletStorage):
    """File-system wallet storage."""

    def __init__(self, base_path: str) -> None:
        self._base = Path(base_path)

    async def get(self, key: str) -> str | None:
        path = self._base / quote(key, safe="")
        try:
            return path.read_text()
        except FileNotFoundError:
            return None

    async def set(self, key: str, value: str) -> None:
        self._base.mkdir(parents=True, exist_ok=True)
        path = self._base / quote(key, safe="")
        path.write_text(value)

    async def delete(self, key: str) -> None:
        path = self._base / quote(key, safe="")
        try:
            path.unlink()
        except FileNotFoundError:
            pass

    async def list_keys(self, prefix: str) -> list[str]:
        if not self._base.exists():
            return []
        return [
            unquote(f.name)
            for f in self._base.iterdir()
            if unquote(f.name).startswith(prefix)
        ]


class CredentialWallet:
    """Credential wallet: stores keys and verifiable credentials."""

    def __init__(self, storage: WalletStorage) -> None:
        self._storage = storage

    # ─── Key Management ─────────────────────────────────────────────────

    async def store_key_pair(self, did: str, key_pair: KeyPair) -> None:
        """Store a key pair."""
        await self._storage.set(
            f"key:{did}",
            json.dumps(
                {
                    "public_key": list(key_pair.public_key),
                    "private_key": list(key_pair.private_key),
                }
            ),
        )

    async def get_key_pair(self, did: str) -> KeyPair | None:
        """Retrieve a key pair by DID."""
        raw = await self._storage.get(f"key:{did}")
        if raw is None:
            return None
        data = json.loads(raw)
        return KeyPair(
            public_key=bytes(data["public_key"]),
            private_key=bytes(data["private_key"]),
        )

    async def delete_key_pair(self, did: str) -> None:
        """Delete a key pair."""
        await self._storage.delete(f"key:{did}")

    async def list_dids(self) -> list[str]:
        """List all stored DIDs."""
        keys = await self._storage.list_keys("key:")
        return [k[4:] for k in keys]

    # ─── Credential Management ──────────────────────────────────────────

    async def store_credential(self, credential: VerifiableCredential) -> None:
        """Store a verifiable credential."""
        await self._storage.set(
            f"cred:{credential.id}",
            credential.model_dump_json(by_alias=True),
        )

    async def get_credential(self, cred_id: str) -> VerifiableCredential | None:
        """Retrieve a credential by ID."""
        raw = await self._storage.get(f"cred:{cred_id}")
        if raw is None:
            return None
        return VerifiableCredential.model_validate_json(raw)

    async def delete_credential(self, cred_id: str) -> None:
        """Delete a credential."""
        await self._storage.delete(f"cred:{cred_id}")

    async def list_credentials(self) -> list[str]:
        """List all stored credential IDs."""
        keys = await self._storage.list_keys("cred:")
        return [k[5:] for k in keys]

    async def get_credentials_by_type(
        self, cred_type: str
    ) -> list[VerifiableCredential]:
        """Get all credentials of a specific type."""
        ids = await self.list_credentials()
        results: list[VerifiableCredential] = []
        for cred_id in ids:
            cred = await self.get_credential(cred_id)
            if cred and cred_type in cred.type:
                results.append(cred)
        return results

    # ─── Metadata ───────────────────────────────────────────────────────

    async def store_meta(self, key: str, value: str) -> None:
        """Store arbitrary metadata."""
        await self._storage.set(f"meta:{key}", value)

    async def get_meta(self, key: str) -> str | None:
        """Retrieve metadata."""
        return await self._storage.get(f"meta:{key}")

    async def delete_meta(self, key: str) -> None:
        """Delete metadata."""
        await self._storage.delete(f"meta:{key}")
