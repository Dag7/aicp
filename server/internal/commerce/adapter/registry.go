package adapter

import (
	"crypto/rand"
	"encoding/hex"
)

// generateID creates a random hex ID for adapter responses.
func generateID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "000000000000"
	}
	return hex.EncodeToString(b)
}

// DefaultRegistry creates a registry pre-loaded with all built-in adapters.
func DefaultRegistry() *Registry {
	reg := NewRegistry()
	reg.Register(NewStripeAdapter())
	reg.Register(NewPayPalAdapter())
	reg.Register(NewUSDCAdapter())
	return reg
}
