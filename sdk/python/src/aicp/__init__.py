"""AICP - Agentic Identity and Commerce Protocol SDK for Python."""

from aicp.client import AICPAgent, AICPClient
from aicp.crypto import (
    KeyPair,
    generate_key_pair,
    get_public_key,
    sign,
    verify,
    sign_json,
    verify_json,
    principal_did,
    agent_did,
    did_identifier,
    parse_did,
    is_principal_did,
    is_agent_did,
    multibase_encode,
    multibase_decode,
    create_proof,
    verify_proof,
    sha256_hash,
    sha256_hex,
    base58_encode,
    base58_decode,
    random_bytes,
)
from aicp.identity import IdentityManager
from aicp.discovery import DiscoveryManager
from aicp.onboarding import OnboardingManager
from aicp.commerce import CommerceManager
from aicp.wallet import CredentialWallet, MemoryWalletStorage, FileWalletStorage
from aicp.provider import AICPServiceAdapter
from aicp.transport import Transport, AICPError

__all__ = [
    # Clients
    "AICPAgent",
    "AICPClient",
    # Managers
    "IdentityManager",
    "DiscoveryManager",
    "OnboardingManager",
    "CommerceManager",
    # Wallet
    "CredentialWallet",
    "MemoryWalletStorage",
    "FileWalletStorage",
    # Provider
    "AICPServiceAdapter",
    # Transport
    "Transport",
    "AICPError",
    # Crypto
    "KeyPair",
    "generate_key_pair",
    "get_public_key",
    "sign",
    "verify",
    "sign_json",
    "verify_json",
    "principal_did",
    "agent_did",
    "did_identifier",
    "parse_did",
    "is_principal_did",
    "is_agent_did",
    "multibase_encode",
    "multibase_decode",
    "create_proof",
    "verify_proof",
    "sha256_hash",
    "sha256_hex",
    "base58_encode",
    "base58_decode",
    "random_bytes",
]
