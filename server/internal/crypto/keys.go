// Package crypto provides cryptographic primitives for the AICP protocol
// including Ed25519 key management, signing, JWS token creation, Merkle
// trees, and multibase encoding.
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

// KeyPair holds an Ed25519 key pair with its DID-derived identifier.
type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// GenerateEd25519KeyPair creates a new random Ed25519 key pair using
// a cryptographically secure random number generator.
func GenerateEd25519KeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating Ed25519 key pair: %w", err)
	}
	return &KeyPair{
		PublicKey:  pub,
		PrivateKey: priv,
	}, nil
}

// KeyPairFromSeed creates an Ed25519 key pair from a 32-byte seed.
// This is deterministic: the same seed always produces the same key pair.
func KeyPairFromSeed(seed []byte) (*KeyPair, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("seed must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	return &KeyPair{
		PublicKey:  pub,
		PrivateKey: priv,
	}, nil
}

// DIDIdentifier computes the AICP DID identifier from a public key.
// It returns the base58btc encoding of the first 16 bytes of SHA-256(publicKey).
func DIDIdentifier(pub ed25519.PublicKey) string {
	hash := sha256.Sum256(pub)
	return Base58Encode(hash[:16])
}

// PrincipalDID constructs a principal DID string from an Ed25519 public key.
func PrincipalDID(pub ed25519.PublicKey) string {
	return "did:aicp:" + DIDIdentifier(pub)
}

// AgentDID constructs an agent DID string from the principal and agent public keys.
func AgentDID(principalPub, agentPub ed25519.PublicKey) string {
	return "did:aicp:" + DIDIdentifier(principalPub) + ":" + DIDIdentifier(agentPub)
}

// PublicKeyMultibase encodes an Ed25519 public key in multibase format
// (z-prefixed base58btc with multicodec ed25519-pub prefix 0xed01).
func PublicKeyMultibase(pub ed25519.PublicKey) string {
	// Multicodec prefix for ed25519-pub: 0xed 0x01
	prefixed := make([]byte, 2+len(pub))
	prefixed[0] = 0xed
	prefixed[1] = 0x01
	copy(prefixed[2:], pub)
	return "z" + Base58Encode(prefixed)
}

// PublicKeyFromMultibase decodes a multibase-encoded Ed25519 public key.
func PublicKeyFromMultibase(multibase string) (ed25519.PublicKey, error) {
	if len(multibase) == 0 || multibase[0] != 'z' {
		return nil, fmt.Errorf("expected z-prefixed base58btc multibase encoding")
	}
	decoded, err := Base58Decode(multibase[1:])
	if err != nil {
		return nil, fmt.Errorf("decoding base58btc: %w", err)
	}
	if len(decoded) < 2 || decoded[0] != 0xed || decoded[1] != 0x01 {
		return nil, fmt.Errorf("invalid multicodec prefix for ed25519-pub")
	}
	pub := ed25519.PublicKey(decoded[2:])
	if len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("decoded key is %d bytes, expected %d", len(pub), ed25519.PublicKeySize)
	}
	return pub, nil
}
