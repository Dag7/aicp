"""Tests for the wallet module."""

import pytest

from aicp.crypto import generate_key_pair
from aicp.types import VerifiableCredential
from aicp.wallet import CredentialWallet, MemoryWalletStorage


@pytest.fixture
def wallet():
    return CredentialWallet(MemoryWalletStorage())


@pytest.fixture
def test_cred():
    return VerifiableCredential.model_validate(
        {
            "@context": ["https://www.w3.org/2018/credentials/v1"],
            "id": "urn:uuid:test-cred-1",
            "type": ["VerifiableCredential", "AICPAgentDelegation"],
            "issuer": "did:aicp:issuer",
            "issuanceDate": "2025-01-01T00:00:00Z",
            "credentialSubject": {"id": "did:aicp:subject"},
        }
    )


class TestMemoryWalletStorage:
    @pytest.mark.asyncio
    async def test_set_and_get(self):
        storage = MemoryWalletStorage()
        await storage.set("key1", "value1")
        assert await storage.get("key1") == "value1"

    @pytest.mark.asyncio
    async def test_missing_returns_none(self):
        storage = MemoryWalletStorage()
        assert await storage.get("missing") is None

    @pytest.mark.asyncio
    async def test_delete(self):
        storage = MemoryWalletStorage()
        await storage.set("key1", "value1")
        await storage.delete("key1")
        assert await storage.get("key1") is None

    @pytest.mark.asyncio
    async def test_list_by_prefix(self):
        storage = MemoryWalletStorage()
        await storage.set("cred:a", "1")
        await storage.set("cred:b", "2")
        await storage.set("key:x", "3")
        result = await storage.list_keys("cred:")
        assert len(result) == 2
        assert "cred:a" in result
        assert "cred:b" in result


class TestKeyManagement:
    @pytest.mark.asyncio
    async def test_store_and_retrieve(self, wallet):
        kp = generate_key_pair()
        await wallet.store_key_pair("did:aicp:test", kp)
        retrieved = await wallet.get_key_pair("did:aicp:test")
        assert retrieved is not None
        assert retrieved.public_key == kp.public_key
        assert retrieved.private_key == kp.private_key

    @pytest.mark.asyncio
    async def test_missing_returns_none(self, wallet):
        assert await wallet.get_key_pair("did:aicp:missing") is None

    @pytest.mark.asyncio
    async def test_delete(self, wallet):
        kp = generate_key_pair()
        await wallet.store_key_pair("did:aicp:test", kp)
        await wallet.delete_key_pair("did:aicp:test")
        assert await wallet.get_key_pair("did:aicp:test") is None

    @pytest.mark.asyncio
    async def test_list_dids(self, wallet):
        kp1 = generate_key_pair()
        kp2 = generate_key_pair()
        await wallet.store_key_pair("did:aicp:a", kp1)
        await wallet.store_key_pair("did:aicp:b", kp2)
        dids = await wallet.list_dids()
        assert len(dids) == 2
        assert "did:aicp:a" in dids
        assert "did:aicp:b" in dids


class TestCredentialManagement:
    @pytest.mark.asyncio
    async def test_store_and_retrieve(self, wallet, test_cred):
        await wallet.store_credential(test_cred)
        retrieved = await wallet.get_credential("urn:uuid:test-cred-1")
        assert retrieved is not None
        assert retrieved.id == "urn:uuid:test-cred-1"
        assert retrieved.issuer == "did:aicp:issuer"

    @pytest.mark.asyncio
    async def test_missing_returns_none(self, wallet):
        assert await wallet.get_credential("missing") is None

    @pytest.mark.asyncio
    async def test_delete(self, wallet, test_cred):
        await wallet.store_credential(test_cred)
        await wallet.delete_credential("urn:uuid:test-cred-1")
        assert await wallet.get_credential("urn:uuid:test-cred-1") is None

    @pytest.mark.asyncio
    async def test_list_credentials(self, wallet, test_cred):
        await wallet.store_credential(test_cred)
        cred2 = test_cred.model_copy(update={"id": "urn:uuid:test-cred-2"})
        await wallet.store_credential(cred2)
        ids = await wallet.list_credentials()
        assert len(ids) == 2

    @pytest.mark.asyncio
    async def test_filter_by_type(self, wallet, test_cred):
        await wallet.store_credential(test_cred)
        spending = test_cred.model_copy(
            update={
                "id": "urn:uuid:spending-1",
                "type": ["VerifiableCredential", "AICPSpendingAuthorization"],
            }
        )
        await wallet.store_credential(spending)

        delegations = await wallet.get_credentials_by_type("AICPAgentDelegation")
        assert len(delegations) == 1
        assert delegations[0].id == "urn:uuid:test-cred-1"


class TestMetadata:
    @pytest.mark.asyncio
    async def test_store_and_retrieve(self, wallet):
        await wallet.store_meta("accountID", "acc_123")
        assert await wallet.get_meta("accountID") == "acc_123"

    @pytest.mark.asyncio
    async def test_missing_returns_none(self, wallet):
        assert await wallet.get_meta("missing") is None

    @pytest.mark.asyncio
    async def test_delete(self, wallet):
        await wallet.store_meta("key", "val")
        await wallet.delete_meta("key")
        assert await wallet.get_meta("key") is None
