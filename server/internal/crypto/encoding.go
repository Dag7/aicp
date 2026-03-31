package crypto

import (
	"github.com/mr-tron/base58"
)

// Base58Encode encodes bytes to base58btc (Bitcoin alphabet).
func Base58Encode(data []byte) string {
	return base58.Encode(data)
}

// Base58Decode decodes a base58btc string to bytes.
func Base58Decode(s string) ([]byte, error) {
	return base58.Decode(s)
}
