package crypto

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Key Generation ---

func TestGenerateEd25519KeyPair(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)
	assert.Len(t, kp.PublicKey, ed25519.PublicKeySize)
	assert.Len(t, kp.PrivateKey, ed25519.PrivateKeySize)
}

func TestGenerateEd25519KeyPair_Uniqueness(t *testing.T) {
	kp1, err := GenerateEd25519KeyPair()
	require.NoError(t, err)
	kp2, err := GenerateEd25519KeyPair()
	require.NoError(t, err)
	assert.NotEqual(t, kp1.PublicKey, kp2.PublicKey)
	assert.NotEqual(t, kp1.PrivateKey, kp2.PrivateKey)
}

func TestKeyPairFromSeed(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}

	kp1, err := KeyPairFromSeed(seed)
	require.NoError(t, err)
	kp2, err := KeyPairFromSeed(seed)
	require.NoError(t, err)

	// Same seed must produce same key pair.
	assert.Equal(t, kp1.PublicKey, kp2.PublicKey)
	assert.Equal(t, kp1.PrivateKey, kp2.PrivateKey)
}

func TestKeyPairFromSeed_InvalidLength(t *testing.T) {
	_, err := KeyPairFromSeed([]byte("too short"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "seed must be")
}

// --- DID Identifiers ---

func TestDIDIdentifier_Deterministic(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	id1 := DIDIdentifier(kp.PublicKey)
	id2 := DIDIdentifier(kp.PublicKey)
	assert.Equal(t, id1, id2)
	assert.NotEmpty(t, id1)
}

func TestPrincipalDID(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	did := PrincipalDID(kp.PublicKey)
	assert.True(t, strings.HasPrefix(did, "did:aicp:"))
	parts := strings.Split(did, ":")
	assert.Len(t, parts, 3) // did:aicp:<id>
}

func TestAgentDID(t *testing.T) {
	principal, err := GenerateEd25519KeyPair()
	require.NoError(t, err)
	agent, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	did := AgentDID(principal.PublicKey, agent.PublicKey)
	assert.True(t, strings.HasPrefix(did, "did:aicp:"))
	parts := strings.Split(did, ":")
	assert.Len(t, parts, 4) // did:aicp:<principal-id>:<agent-id>
}

// --- Multibase Encoding ---

func TestPublicKeyMultibase_RoundTrip(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	encoded := PublicKeyMultibase(kp.PublicKey)
	assert.True(t, strings.HasPrefix(encoded, "z"))

	decoded, err := PublicKeyFromMultibase(encoded)
	require.NoError(t, err)
	assert.Equal(t, kp.PublicKey, decoded)
}

func TestPublicKeyFromMultibase_InvalidPrefix(t *testing.T) {
	_, err := PublicKeyFromMultibase("invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "z-prefixed")
}

func TestPublicKeyFromMultibase_EmptyString(t *testing.T) {
	_, err := PublicKeyFromMultibase("")
	assert.Error(t, err)
}

// --- Signing ---

func TestSignVerify(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	message := []byte("hello aicp")
	sig := Sign(kp.PrivateKey, message)
	assert.True(t, Verify(kp.PublicKey, message, sig))
}

func TestVerify_WrongKey(t *testing.T) {
	kp1, err := GenerateEd25519KeyPair()
	require.NoError(t, err)
	kp2, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	message := []byte("hello aicp")
	sig := Sign(kp1.PrivateKey, message)
	assert.False(t, Verify(kp2.PublicKey, message, sig))
}

func TestVerify_TamperedMessage(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	sig := Sign(kp.PrivateKey, []byte("original"))
	assert.False(t, Verify(kp.PublicKey, []byte("tampered"), sig))
}

func TestSignJSON_VerifyJSON(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	data := map[string]string{"action": "pay", "amount": "100"}
	jsonBytes, err := json.Marshal(data)
	require.NoError(t, err)

	sigB58, err := SignJSON(kp.PrivateKey, jsonBytes)
	require.NoError(t, err)
	assert.NotEmpty(t, sigB58)

	valid, err := VerifyJSON(kp.PublicKey, jsonBytes, sigB58)
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestVerifyJSON_InvalidSignature(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	jsonBytes := []byte(`{"test": true}`)

	// Valid base58 but wrong signature bytes — should return false, no error.
	wrongSig := Base58Encode([]byte("this is not a valid ed25519 signature but is valid base58"))
	valid, err := VerifyJSON(kp.PublicKey, jsonBytes, wrongSig)
	require.NoError(t, err)
	assert.False(t, valid)

	// Invalid base58 — should return error.
	_, err = VerifyJSON(kp.PublicKey, jsonBytes, "0OIl-invalid")
	assert.Error(t, err)
}

// --- SHA256 ---

func TestSHA256Hex(t *testing.T) {
	hash := SHA256Hex([]byte("test"))
	assert.Len(t, hash, 64) // 32 bytes = 64 hex chars
	// Known SHA-256 of "test".
	assert.Equal(t, "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", hash)
}

func TestSHA256Bytes(t *testing.T) {
	hash := SHA256Bytes([]byte("test"))
	assert.Equal(t, SHA256Hex([]byte("test")), hex.EncodeToString(hash[:]))
}

// --- Base58 Encoding ---

func TestBase58_RoundTrip(t *testing.T) {
	original := []byte("AICP protocol test data")
	encoded := Base58Encode(original)
	decoded, err := Base58Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestBase58Decode_Invalid(t *testing.T) {
	_, err := Base58Decode("0OIl") // Contains invalid base58 characters.
	assert.Error(t, err)
}

// --- Merkle Tree ---

func TestMerkleRoot_Empty(t *testing.T) {
	root, err := MerkleRoot([]string{})
	require.NoError(t, err)
	assert.Equal(t, SHA256Hex([]byte("empty")), root)
}

func TestMerkleRoot_SingleLeaf(t *testing.T) {
	leaf := SHA256Hex([]byte("leaf0"))
	root, err := MerkleRoot([]string{leaf})
	require.NoError(t, err)
	assert.Equal(t, leaf, root)
}

func TestMerkleRoot_TwoLeaves(t *testing.T) {
	leaf0 := SHA256Hex([]byte("leaf0"))
	leaf1 := SHA256Hex([]byte("leaf1"))
	root, err := MerkleRoot([]string{leaf0, leaf1})
	require.NoError(t, err)
	assert.NotEqual(t, leaf0, root)
	assert.NotEqual(t, leaf1, root)
	assert.Len(t, root, 64)
}

func TestMerkleRoot_Deterministic(t *testing.T) {
	leaves := []string{
		SHA256Hex([]byte("a")),
		SHA256Hex([]byte("b")),
		SHA256Hex([]byte("c")),
		SHA256Hex([]byte("d")),
	}
	root1, err := MerkleRoot(leaves)
	require.NoError(t, err)
	root2, err := MerkleRoot(leaves)
	require.NoError(t, err)
	assert.Equal(t, root1, root2)
}

func TestMerkleRoot_OddLeaves(t *testing.T) {
	leaves := []string{
		SHA256Hex([]byte("a")),
		SHA256Hex([]byte("b")),
		SHA256Hex([]byte("c")),
	}
	root, err := MerkleRoot([]string{leaves[0], leaves[1], leaves[2]})
	require.NoError(t, err)
	assert.Len(t, root, 64)
}

func TestMerkleRoot_OrderMatters(t *testing.T) {
	leaf0 := SHA256Hex([]byte("first"))
	leaf1 := SHA256Hex([]byte("second"))

	root1, err := MerkleRoot([]string{leaf0, leaf1})
	require.NoError(t, err)
	root2, err := MerkleRoot([]string{leaf1, leaf0})
	require.NoError(t, err)
	assert.NotEqual(t, root1, root2)
}

func TestMerkleVerifyProof_TwoLeaves(t *testing.T) {
	leaf0 := SHA256Hex([]byte("leaf0"))
	leaf1 := SHA256Hex([]byte("leaf1"))
	root, err := MerkleRoot([]string{leaf0, leaf1})
	require.NoError(t, err)

	// For a 2-leaf tree, the proof for leaf0 (index=0) is [leaf1].
	valid, err := MerkleVerifyProof(leaf0, []string{leaf1}, 0, root)
	require.NoError(t, err)
	assert.True(t, valid)

	// Proof for leaf1 (index=1) is [leaf0].
	valid, err = MerkleVerifyProof(leaf1, []string{leaf0}, 1, root)
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestMerkleVerifyProof_InvalidProof(t *testing.T) {
	leaf0 := SHA256Hex([]byte("leaf0"))
	leaf1 := SHA256Hex([]byte("leaf1"))
	root, err := MerkleRoot([]string{leaf0, leaf1})
	require.NoError(t, err)

	fakeProof := SHA256Hex([]byte("fake"))
	valid, err := MerkleVerifyProof(leaf0, []string{fakeProof}, 0, root)
	require.NoError(t, err)
	assert.False(t, valid)
}

func TestMerkleRoot_InvalidHex(t *testing.T) {
	_, err := MerkleRoot([]string{"not-valid-hex"})
	assert.Error(t, err)
}

// --- JWS ---

func TestJWS_SignAndVerify(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	keyID := "did:aicp:test123#key-1"
	signer, err := NewJWSSigner(kp.PrivateKey, keyID)
	require.NoError(t, err)

	claims := map[string]any{
		"iss":     "did:aicp:test123",
		"sub":     "payment",
		"amount":  "10.00",
		"currency": "USD",
	}

	token, err := signer.SignClaims(claims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	// JWS compact serialization has 3 parts separated by dots.
	parts := strings.Split(token, ".")
	assert.Len(t, parts, 3)

	// Verify.
	verifier := NewJWSVerifier()
	var decoded map[string]any
	err = verifier.VerifyClaims(token, kp.PublicKey, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "did:aicp:test123", decoded["iss"])
	assert.Equal(t, "payment", decoded["sub"])
	assert.Equal(t, "10.00", decoded["amount"])
}

func TestJWS_VerifyWithWrongKey(t *testing.T) {
	kp1, err := GenerateEd25519KeyPair()
	require.NoError(t, err)
	kp2, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	signer, err := NewJWSSigner(kp1.PrivateKey, "did:aicp:test#key-1")
	require.NoError(t, err)

	token, err := signer.SignClaims(map[string]string{"test": "value"})
	require.NoError(t, err)

	verifier := NewJWSVerifier()
	var decoded map[string]string
	err = verifier.VerifyClaims(token, kp2.PublicKey, &decoded)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "verifying JWS signature")
}

func TestJWS_ExtractKeyID(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	keyID := "did:aicp:abc123#key-1"
	signer, err := NewJWSSigner(kp.PrivateKey, keyID)
	require.NoError(t, err)

	token, err := signer.SignClaims(map[string]string{"test": "value"})
	require.NoError(t, err)

	extracted, err := ExtractKeyID(token)
	require.NoError(t, err)
	assert.Equal(t, keyID, extracted)
}

func TestJWS_ExtractKeyID_InvalidToken(t *testing.T) {
	_, err := ExtractKeyID("not.a.valid.token")
	assert.Error(t, err)
}

func TestJWS_VerifyClaimsWithExpiry_NotExpired(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	signer, err := NewJWSSigner(kp.PrivateKey, "did:aicp:test#key-1")
	require.NoError(t, err)

	// Claims without expiry should pass.
	token, err := signer.SignClaims(map[string]string{"test": "value"})
	require.NoError(t, err)

	verifier := NewJWSVerifier()
	var decoded map[string]string
	err = verifier.VerifyClaimsWithExpiry(token, kp.PublicKey, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "value", decoded["test"])
}

func TestJWS_VerifyClaimsWithExpiry_Expired(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	signer, err := NewJWSSigner(kp.PrivateKey, "did:aicp:test#key-1")
	require.NoError(t, err)

	// Claims with a past expiry.
	claims := map[string]any{
		"test": "value",
		"exp":  0, // Unix epoch = expired.
	}
	token, err := signer.SignClaims(claims)
	require.NoError(t, err)

	verifier := NewJWSVerifier()
	var decoded map[string]any
	err = verifier.VerifyClaimsWithExpiry(token, kp.PublicKey, &decoded)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token expired")
}

// --- Standalone SignJWS / VerifyJWS ---

func TestSignJWS_AndVerifyJWS(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	payload := []byte(`{"sub":"did:aicp:test","iss":"https://idp.example.com"}`)

	token, err := SignJWS(kp.PrivateKey, payload, "JWT")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	decoded, err := VerifyJWS(kp.PublicKey, token)
	require.NoError(t, err)

	var claims map[string]string
	require.NoError(t, json.Unmarshal(decoded, &claims))
	assert.Equal(t, "did:aicp:test", claims["sub"])
	assert.Equal(t, "https://idp.example.com", claims["iss"])
}

func TestSignJWS_EmptyType(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	token, err := SignJWS(kp.PrivateKey, []byte("hello"), "")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	decoded, err := VerifyJWS(kp.PublicKey, token)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), decoded)
}

func TestVerifyJWS_WrongKey(t *testing.T) {
	kp1, _ := GenerateEd25519KeyPair()
	kp2, _ := GenerateEd25519KeyPair()

	token, err := SignJWS(kp1.PrivateKey, []byte("secret"), "JWT")
	require.NoError(t, err)

	_, err = VerifyJWS(kp2.PublicKey, token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "verifying JWS signature")
}

func TestVerifyJWS_InvalidToken(t *testing.T) {
	kp, _ := GenerateEd25519KeyPair()

	_, err := VerifyJWS(kp.PublicKey, "not-a-jws-token")
	assert.Error(t, err)
}

func TestSignJWS_DifferentTypes(t *testing.T) {
	kp, _ := GenerateEd25519KeyPair()

	types := []string{"JWT", "aicp-payment-token+jws", "at+jwt", ""}
	for _, typ := range types {
		token, err := SignJWS(kp.PrivateKey, []byte(`{"test":true}`), typ)
		require.NoError(t, err, "type: %s", typ)

		decoded, err := VerifyJWS(kp.PublicKey, token)
		require.NoError(t, err, "type: %s", typ)
		assert.Contains(t, string(decoded), "test")
	}
}
