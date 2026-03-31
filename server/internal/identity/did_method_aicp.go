package identity

import (
	"fmt"
	"strings"
)

const (
	// DIDMethodPrefix is the prefix for all AICP DIDs.
	DIDMethodPrefix = "did:aicp:"
)

// ParseDID breaks an AICP DID string into its components.
// Returns (method, principalID, agentID, error).
// Principal DID: did:aicp:<principal-id> -> ("aicp", "<principal-id>", "", nil)
// Agent DID:     did:aicp:<principal-id>:<agent-id> -> ("aicp", "<principal-id>", "<agent-id>", nil)
func ParseDID(did string) (method, principalID, agentID string, err error) {
	if !strings.HasPrefix(did, "did:") {
		return "", "", "", fmt.Errorf("invalid DID: must start with 'did:'")
	}

	parts := strings.Split(did, ":")
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf("invalid DID: insufficient parts")
	}

	method = parts[1]
	if method != "aicp" {
		return "", "", "", fmt.Errorf("unsupported DID method: %s", method)
	}

	principalID = parts[2]
	if principalID == "" {
		return "", "", "", fmt.Errorf("invalid DID: empty principal identifier")
	}

	if len(parts) == 4 {
		agentID = parts[3]
		if agentID == "" {
			return "", "", "", fmt.Errorf("invalid DID: empty agent identifier")
		}
	} else if len(parts) > 4 {
		return "", "", "", fmt.Errorf("invalid DID: too many parts")
	}

	return method, principalID, agentID, nil
}

// IsPrincipalDID returns true if the DID is a principal DID (3 parts).
func IsPrincipalDID(did string) bool {
	_, _, agentID, err := ParseDID(did)
	return err == nil && agentID == ""
}

// IsAgentDID returns true if the DID is an agent DID (4 parts).
func IsAgentDID(did string) bool {
	_, _, agentID, err := ParseDID(did)
	return err == nil && agentID != ""
}

// PrincipalDIDFromAgent extracts the principal DID from an agent DID.
// If the input is already a principal DID, it returns it unchanged.
func PrincipalDIDFromAgent(did string) (string, error) {
	_, principalID, _, err := ParseDID(did)
	if err != nil {
		return "", err
	}
	return "did:aicp:" + principalID, nil
}

// ValidateDID checks that a DID string is well-formed.
func ValidateDID(did string) error {
	_, _, _, err := ParseDID(did)
	return err
}
