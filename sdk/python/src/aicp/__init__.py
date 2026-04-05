"""AICP - Agentic Identity and Commerce Protocol SDK for Python."""

from aicp.client import AICPAgent, AICPClient
from aicp.commerce import CommerceManager
from aicp.crypto import (
    KeyPair,
    agent_did,
    base58_decode,
    base58_encode,
    create_proof,
    did_identifier,
    generate_key_pair,
    get_public_key,
    is_agent_did,
    is_principal_did,
    multibase_decode,
    multibase_encode,
    parse_did,
    principal_did,
    random_bytes,
    sha256_hash,
    sha256_hex,
    sign,
    sign_json,
    verify,
    verify_json,
    verify_proof,
)
from aicp.discovery import DiscoveryManager
from aicp.identity import IdentityManager
from aicp.onboarding import OnboardingManager
from aicp.provider import AICPServiceAdapter
from aicp.transport import AICPError, Transport
from aicp.wallet import CredentialWallet, FileWalletStorage, MemoryWalletStorage

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
