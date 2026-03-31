package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Sign signs a message with the given Ed25519 private key.
func Sign(privateKey ed25519.PrivateKey, message []byte) []byte {
	return ed25519.Sign(privateKey, message)
}

// Verify verifies an Ed25519 signature over a message.
func Verify(publicKey ed25519.PublicKey, message, signature []byte) bool {
	return ed25519.Verify(publicKey, message, signature)
}

// SignJSON signs the canonical JSON representation of data.
// It first computes SHA-256 of the JSON bytes, then signs the hash.
func SignJSON(privateKey ed25519.PrivateKey, jsonBytes []byte) (string, error) {
	hash := sha256.Sum256(jsonBytes)
	sig := Sign(privateKey, hash[:])
	return Base58Encode(sig), nil
}

// VerifyJSON verifies a base58-encoded signature over JSON data.
func VerifyJSON(publicKey ed25519.PublicKey, jsonBytes []byte, signatureB58 string) (bool, error) {
	sig, err := Base58Decode(signatureB58)
	if err != nil {
		return false, fmt.Errorf("decoding signature: %w", err)
	}
	hash := sha256.Sum256(jsonBytes)
	return Verify(publicKey, hash[:], sig), nil
}

// SHA256Hex computes the SHA-256 hash of data and returns it as a hex string.
func SHA256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// SHA256Bytes computes the SHA-256 hash of data.
func SHA256Bytes(data []byte) [32]byte {
	return sha256.Sum256(data)
}
