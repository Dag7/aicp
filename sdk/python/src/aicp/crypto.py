"""Ed25519 cryptographic operations for AICP."""

from __future__ import annotations

import hashlib
import json
import os
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any

import nacl.signing
import nacl.utils

from aicp.types import Proof

BASE58_ALPHABET = b"123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"


@dataclass(frozen=True, slots=True)
class KeyPair:
    """An Ed25519 key pair."""

    public_key: bytes
    private_key: bytes


# ─── Key Operations ─────────────────────────────────────────────────────────


def generate_key_pair() -> KeyPair:
    """Generate a new Ed25519 key pair."""
    signing_key = nacl.signing.SigningKey.generate()
    return KeyPair(
        public_key=bytes(signing_key.verify_key),
        private_key=bytes(signing_key),
    )


def get_public_key(private_key: bytes) -> bytes:
    """Derive the public key from a private key."""
    signing_key = nacl.signing.SigningKey(private_key)
    return bytes(signing_key.verify_key)


# ─── Signing ────────────────────────────────────────────────────────────────


def sign(message: bytes, private_key: bytes) -> bytes:
    """Sign a message with an Ed25519 private key. Returns a 64-byte signature."""
    signing_key = nacl.signing.SigningKey(private_key)
    signed = signing_key.sign(message)
    return signed.signature


def verify(signature: bytes, message: bytes, public_key: bytes) -> bool:
    """Verify an Ed25519 signature."""
    verify_key = nacl.signing.VerifyKey(public_key)
    try:
        verify_key.verify(message, signature)
        return True
    except nacl.exceptions.BadSignatureError:
        return False


def sign_json(data: Any, private_key: bytes) -> str:
    """Sign a JSON-serializable object. Returns base58-encoded signature."""
    json_bytes = json.dumps(data, separators=(",", ":"), sort_keys=False).encode()
    digest = hashlib.sha256(json_bytes).digest()
    sig = sign(digest, private_key)
    return base58_encode(sig)


def verify_json(data: Any, signature_base58: str, public_key: bytes) -> bool:
    """Verify a base58-encoded signature over a JSON object."""
    sig = base58_decode(signature_base58)
    json_bytes = json.dumps(data, separators=(",", ":"), sort_keys=False).encode()
    digest = hashlib.sha256(json_bytes).digest()
    return verify(sig, digest, public_key)


# ─── DID Operations ─────────────────────────────────────────────────────────


def did_identifier(public_key: bytes) -> str:
    """Create a DID identifier from a public key (base58btc of first 16 bytes of SHA-256)."""
    digest = hashlib.sha256(public_key).digest()
    return base58_encode(digest[:16])


def principal_did(public_key: bytes) -> str:
    """Build a principal DID from a public key."""
    return f"did:aicp:{did_identifier(public_key)}"


def agent_did(principal_pub_key: bytes, agent_pub_key: bytes) -> str:
    """Build an agent DID from the principal and agent public keys."""
    return f"did:aicp:{did_identifier(principal_pub_key)}:{did_identifier(agent_pub_key)}"


def parse_did(did: str) -> dict[str, str]:
    """Parse a did:aicp DID. Returns dict with 'method', 'principal_id', and optionally 'agent_id'."""
    parts = did.split(":")
    if len(parts) < 3 or parts[0] != "did" or parts[1] != "aicp":
        raise ValueError(f"invalid AICP DID: {did}")
    result: dict[str, str] = {"method": "aicp", "principal_id": parts[2]}
    if len(parts) >= 4:
        result["agent_id"] = parts[3]
    return result


def is_principal_did(did: str) -> bool:
    """Check if a DID is a principal DID (no agent segment)."""
    parts = did.split(":")
    return len(parts) == 3 and parts[0] == "did" and parts[1] == "aicp"


def is_agent_did(did: str) -> bool:
    """Check if a DID is an agent DID (has agent segment)."""
    parts = did.split(":")
    return len(parts) == 4 and parts[0] == "did" and parts[1] == "aicp"


# ─── Multibase Encoding ─────────────────────────────────────────────────────


def multibase_encode(data: bytes) -> str:
    """Encode bytes as multibase (base58btc with 'z' prefix)."""
    return "z" + base58_encode(data)


def multibase_decode(encoded: str) -> bytes:
    """Decode a multibase string (expects 'z' prefix for base58btc)."""
    if not encoded.startswith("z"):
        raise ValueError("unsupported multibase encoding: expected 'z' prefix")
    return base58_decode(encoded[1:])


# ─── Proof Construction ─────────────────────────────────────────────────────


def create_proof(
    data: Any,
    private_key: bytes,
    verification_method_id: str,
    purpose: str = "assertionMethod",
    *,
    challenge: str | None = None,
    domain: str | None = None,
) -> Proof:
    """Create a proof over JSON data."""
    proof_value = sign_json(data, private_key)
    return Proof(
        type="Ed25519Signature2020",
        created=datetime.now(timezone.utc).isoformat(),
        verificationMethod=verification_method_id,
        proofPurpose=purpose,
        proofValue=proof_value,
        challenge=challenge,
        domain=domain,
    )


def verify_proof(data: Any, proof: Proof, public_key: bytes) -> bool:
    """Verify a proof over JSON data."""
    return verify_json(data, proof.proof_value, public_key)


# ─── SHA-256 ────────────────────────────────────────────────────────────────


def sha256_hash(data: bytes) -> bytes:
    """Compute SHA-256 hash."""
    return hashlib.sha256(data).digest()


def sha256_hex(data: str) -> str:
    """Compute SHA-256 of a string. Returns hex."""
    return hashlib.sha256(data.encode()).hexdigest()


# ─── Base58 ─────────────────────────────────────────────────────────────────


def base58_encode(data: bytes) -> str:
    """Encode bytes as base58 (Bitcoin alphabet)."""
    if not data:
        return ""

    # Count leading zeros.
    zeros = 0
    for byte in data:
        if byte == 0:
            zeros += 1
        else:
            break

    # Convert to integer and then to base58.
    num = int.from_bytes(data, "big")
    result: list[int] = []
    while num > 0:
        num, rem = divmod(num, 58)
        result.append(rem)
    result.reverse()

    return "1" * zeros + "".join(chr(BASE58_ALPHABET[i]) for i in result)


def base58_decode(s: str) -> bytes:
    """Decode a base58 string to bytes."""
    if not s:
        return b""

    # Count leading '1's (zeros).
    zeros = 0
    for ch in s:
        if ch == "1":
            zeros += 1
        else:
            break

    # Convert from base58 to integer.
    num = 0
    for ch in s:
        idx = BASE58_ALPHABET.find(ch.encode())
        if idx < 0:
            raise ValueError(f"invalid base58 character: {ch}")
        num = num * 58 + idx

    if num == 0:
        return b"\x00" * zeros

    # Convert integer to bytes.
    result = num.to_bytes((num.bit_length() + 7) // 8, "big")
    return b"\x00" * zeros + result


# ─── Random ─────────────────────────────────────────────────────────────────


def random_bytes(length: int) -> bytes:
    """Generate cryptographically random bytes."""
    return os.urandom(length)
