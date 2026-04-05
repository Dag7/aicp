"""Tests for the crypto module."""

import pytest

from aicp.crypto import (
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


class TestKeyOperations:
    def test_generate_key_pair(self):
        kp = generate_key_pair()
        assert len(kp.public_key) == 32
        assert len(kp.private_key) == 32

    def test_derive_public_key(self):
        kp = generate_key_pair()
        derived = get_public_key(kp.private_key)
        assert derived == kp.public_key

    def test_unique_key_pairs(self):
        kp1 = generate_key_pair()
        kp2 = generate_key_pair()
        assert kp1.public_key != kp2.public_key
        assert kp1.private_key != kp2.private_key


class TestSigning:
    def test_sign_and_verify(self):
        kp = generate_key_pair()
        message = b"hello AICP"
        sig = sign(message, kp.private_key)
        assert len(sig) == 64
        assert verify(sig, message, kp.public_key)

    def test_wrong_key_rejects(self):
        kp1 = generate_key_pair()
        kp2 = generate_key_pair()
        message = b"test"
        sig = sign(message, kp1.private_key)
        assert not verify(sig, message, kp2.public_key)

    def test_modified_message_rejects(self):
        kp = generate_key_pair()
        sig = sign(b"original", kp.private_key)
        assert not verify(sig, b"modified", kp.public_key)


class TestJSONSigning:
    def test_sign_and_verify_json(self):
        kp = generate_key_pair()
        data = {"foo": "bar", "num": 42}
        sig = sign_json(data, kp.private_key)
        assert isinstance(sig, str)
        assert verify_json(data, sig, kp.public_key)

    def test_wrong_key_rejects(self):
        kp1 = generate_key_pair()
        kp2 = generate_key_pair()
        data = {"test": True}
        sig = sign_json(data, kp1.private_key)
        assert not verify_json(data, sig, kp2.public_key)

    def test_modified_data_rejects(self):
        kp = generate_key_pair()
        sig = sign_json({"key": "value"}, kp.private_key)
        assert not verify_json({"key": "changed"}, sig, kp.public_key)


class TestDID:
    def test_principal_did(self):
        kp = generate_key_pair()
        did = principal_did(kp.public_key)
        assert did.startswith("did:aicp:")
        assert is_principal_did(did)
        assert not is_agent_did(did)

    def test_agent_did(self):
        p_kp = generate_key_pair()
        a_kp = generate_key_pair()
        did = agent_did(p_kp.public_key, a_kp.public_key)
        parts = did.split(":")
        assert len(parts) == 4
        assert is_agent_did(did)
        assert not is_principal_did(did)

    def test_parse_principal_did(self):
        kp = generate_key_pair()
        did = principal_did(kp.public_key)
        parsed = parse_did(did)
        assert parsed["method"] == "aicp"
        assert "principal_id" in parsed
        assert "agent_id" not in parsed

    def test_parse_agent_did(self):
        p_kp = generate_key_pair()
        a_kp = generate_key_pair()
        did = agent_did(p_kp.public_key, a_kp.public_key)
        parsed = parse_did(did)
        assert parsed["method"] == "aicp"
        assert "agent_id" in parsed

    def test_invalid_did(self):
        with pytest.raises(ValueError, match="invalid AICP DID"):
            parse_did("not-a-did")
        with pytest.raises(ValueError, match="invalid AICP DID"):
            parse_did("did:other:123")

    def test_did_identifier_deterministic(self):
        kp = generate_key_pair()
        id1 = did_identifier(kp.public_key)
        id2 = did_identifier(kp.public_key)
        assert id1 == id2


class TestMultibase:
    def test_round_trip(self):
        data = random_bytes(32)
        encoded = multibase_encode(data)
        assert encoded.startswith("z")
        decoded = multibase_decode(encoded)
        assert decoded == data

    def test_unsupported_prefix(self):
        with pytest.raises(ValueError, match="unsupported multibase"):
            multibase_decode("mabc")


class TestBase58:
    def test_round_trip(self):
        data = bytes([1, 2, 3, 4, 5])
        encoded = base58_encode(data)
        decoded = base58_decode(encoded)
        assert decoded == data

    def test_leading_zeros(self):
        data = bytes([0, 0, 0, 1, 2])
        encoded = base58_encode(data)
        assert encoded.startswith("111")
        decoded = base58_decode(encoded)
        assert decoded == data

    def test_empty(self):
        assert base58_encode(b"") == ""
        assert base58_decode("") == b""

    def test_invalid_character(self):
        with pytest.raises(ValueError, match="invalid base58 character"):
            base58_decode("0OIl")


class TestProof:
    def test_create_and_verify(self):
        kp = generate_key_pair()
        data = {"message": "test"}
        proof = create_proof(data, kp.private_key, "did:aicp:test#key-1")
        assert proof.type == "Ed25519Signature2020"
        assert proof.verification_method == "did:aicp:test#key-1"
        assert proof.proof_purpose == "assertionMethod"
        assert verify_proof(data, proof, kp.public_key)

    def test_with_challenge_and_domain(self):
        kp = generate_key_pair()
        data = {"challenge": "abc123"}
        proof = create_proof(
            data,
            kp.private_key,
            "did:aicp:test#key-1",
            "authentication",
            challenge="abc123",
            domain="example.com",
        )
        assert proof.proof_purpose == "authentication"
        assert proof.challenge == "abc123"
        assert proof.domain == "example.com"

    def test_wrong_key_rejects(self):
        kp1 = generate_key_pair()
        kp2 = generate_key_pair()
        data = {"test": True}
        proof = create_proof(data, kp1.private_key, "did:aicp:test#key-1")
        assert not verify_proof(data, proof, kp2.public_key)


class TestHash:
    def test_sha256_consistent(self):
        data = b"hello"
        h1 = sha256_hash(data)
        h2 = sha256_hash(data)
        assert h1 == h2
        assert len(h1) == 32

    def test_sha256_hex_known(self):
        result = sha256_hex("hello")
        assert result == "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"


class TestRandom:
    def test_correct_length(self):
        r = random_bytes(32)
        assert len(r) == 32

    def test_different_each_time(self):
        r1 = random_bytes(16)
        r2 = random_bytes(16)
        assert r1 != r2
