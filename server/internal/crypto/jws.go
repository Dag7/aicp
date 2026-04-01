package crypto

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// JWSSigner creates signed JWS tokens using Ed25519.
type JWSSigner struct {
	signer jose.Signer
	keyID  string
}

// NewJWSSigner creates a new JWS signer with the given Ed25519 private key and key ID.
// The key ID is typically the DID#keyFragment of the signing key.
func NewJWSSigner(privateKey ed25519.PrivateKey, keyID string) (*JWSSigner, error) {
	signingKey := jose.SigningKey{
		Algorithm: jose.EdDSA,
		Key:       privateKey,
	}

	opts := &jose.SignerOptions{}
	opts.WithType("aicp-payment-token+jws")
	opts.WithHeader("kid", keyID)

	signer, err := jose.NewSigner(signingKey, opts)
	if err != nil {
		return nil, fmt.Errorf("creating JWS signer: %w", err)
	}

	return &JWSSigner{
		signer: signer,
		keyID:  keyID,
	}, nil
}

// SignClaims signs an arbitrary claims payload and returns the JWS compact serialization.
func (s *JWSSigner) SignClaims(claims any) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshaling claims: %w", err)
	}

	jws, err := s.signer.Sign(payload)
	if err != nil {
		return "", fmt.Errorf("signing JWS: %w", err)
	}

	compact, err := jws.CompactSerialize()
	if err != nil {
		return "", fmt.Errorf("serializing JWS: %w", err)
	}

	return compact, nil
}

// JWSVerifier verifies JWS tokens using Ed25519 public keys.
type JWSVerifier struct{}

// NewJWSVerifier creates a new JWS verifier.
func NewJWSVerifier() *JWSVerifier {
	return &JWSVerifier{}
}

// VerifyClaims parses and verifies a JWS compact serialization, returning the claims.
// It verifies the Ed25519 signature against the provided public key.
func (v *JWSVerifier) VerifyClaims(token string, publicKey ed25519.PublicKey, dest any) error {
	parsed, err := jose.ParseSigned(token, []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil {
		return fmt.Errorf("parsing JWS: %w", err)
	}

	payload, err := parsed.Verify(publicKey)
	if err != nil {
		return fmt.Errorf("verifying JWS signature: %w", err)
	}

	if err := json.Unmarshal(payload, dest); err != nil {
		return fmt.Errorf("unmarshaling claims: %w", err)
	}

	return nil
}

// VerifyClaimsWithExpiry parses, verifies the signature, and checks expiration.
func (v *JWSVerifier) VerifyClaimsWithExpiry(token string, publicKey ed25519.PublicKey, dest any) error {
	if err := v.VerifyClaims(token, publicKey, dest); err != nil {
		return err
	}

	// Try to extract standard JWT claims for expiry check.
	parsed, err := jose.ParseSigned(token, []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil {
		return err
	}

	payload, err := parsed.Verify(publicKey)
	if err != nil {
		return err
	}

	var stdClaims jwt.Claims
	if err := json.Unmarshal(payload, &stdClaims); err == nil {
		if stdClaims.Expiry != nil {
			if stdClaims.Expiry.Time().Before(time.Now()) {
				return fmt.Errorf("token expired at %s", stdClaims.Expiry.Time())
			}
		}
	}

	return nil
}

// SignJWS signs a payload with Ed25519 and returns JWS compact serialization.
// The typ parameter sets the JWS "typ" header (e.g., "JWT" for OIDC tokens).
func SignJWS(privateKey ed25519.PrivateKey, payload []byte, typ string) (string, error) {
	signingKey := jose.SigningKey{
		Algorithm: jose.EdDSA,
		Key:       privateKey,
	}
	opts := &jose.SignerOptions{}
	if typ != "" {
		opts.WithType(jose.ContentType(typ))
	}
	signer, err := jose.NewSigner(signingKey, opts)
	if err != nil {
		return "", fmt.Errorf("creating JWS signer: %w", err)
	}
	signed, err := signer.Sign(payload)
	if err != nil {
		return "", fmt.Errorf("signing JWS: %w", err)
	}
	compact, err := signed.CompactSerialize()
	if err != nil {
		return "", fmt.Errorf("serializing JWS: %w", err)
	}
	return compact, nil
}

// VerifyJWS verifies a JWS compact serialization against an Ed25519 public key
// and returns the raw payload bytes.
func VerifyJWS(publicKey ed25519.PublicKey, token string) ([]byte, error) {
	parsed, err := jose.ParseSigned(token, []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil {
		return nil, fmt.Errorf("parsing JWS: %w", err)
	}
	payload, err := parsed.Verify(publicKey)
	if err != nil {
		return nil, fmt.Errorf("verifying JWS signature: %w", err)
	}
	return payload, nil
}

// ExtractKeyID returns the key ID from the JWS header without verifying the signature.
func ExtractKeyID(token string) (string, error) {
	parsed, err := jose.ParseSigned(token, []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil {
		return "", fmt.Errorf("parsing JWS: %w", err)
	}

	if len(parsed.Signatures) == 0 {
		return "", fmt.Errorf("no signatures in JWS")
	}

	kid := parsed.Signatures[0].Header.KeyID
	if kid == "" {
		return "", fmt.Errorf("no key ID in JWS header")
	}

	return kid, nil
}
