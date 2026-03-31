// Package aicp provides public types, error codes, and constants for the
// Agentic Identity and Commerce Protocol.
package aicp

// Protocol version constants.
const (
	ProtocolVersion = "1.0"
	ProtocolName    = "AICP"
	DIDMethod       = "aicp"
)

// Build-time variables injected via -ldflags.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)
