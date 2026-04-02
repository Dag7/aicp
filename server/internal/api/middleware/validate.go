package middleware

import (
	"regexp"
	"strings"
)

// didPattern validates the basic format of an AICP DID.
// Principal: did:aicp:<base58btc-id>
// Agent: did:aicp:<principal-id>:<agent-id>
var didPattern = regexp.MustCompile(`^did:aicp:[1-9A-HJ-NP-Za-km-z]{1,44}(:[1-9A-HJ-NP-Za-km-z]{1,44})?$`)

// IsValidDID returns true if the string is a well-formed did:aicp identifier.
func IsValidDID(did string) bool {
	if !strings.HasPrefix(did, "did:aicp:") {
		return false
	}
	return didPattern.MatchString(did)
}

// IsAgentDID returns true if the DID is an agent DID (has two segments after method).
func IsAgentDID(did string) bool {
	if !IsValidDID(did) {
		return false
	}
	// did:aicp:<principal>:<agent> has 4 colon-separated parts.
	return strings.Count(did, ":") == 3
}

// IsPrincipalDID returns true if the DID is a principal DID (single segment after method).
func IsPrincipalDID(did string) bool {
	if !IsValidDID(did) {
		return false
	}
	return strings.Count(did, ":") == 2
}

// PrincipalFromAgentDID extracts the principal DID from an agent DID.
// Returns empty string if the input is not a valid agent DID.
func PrincipalFromAgentDID(agentDID string) string {
	if !IsAgentDID(agentDID) {
		return ""
	}
	// did:aicp:<principal-id>:<agent-id> → did:aicp:<principal-id>
	lastColon := strings.LastIndex(agentDID, ":")
	if lastColon < 0 {
		return ""
	}
	return agentDID[:lastColon]
}

// IsControllerOf returns true if controllerDID is the principal that controls targetDID.
// A principal controls an agent if the agent DID is prefixed with the principal DID.
func IsControllerOf(controllerDID, targetDID string) bool {
	if controllerDID == "" || targetDID == "" {
		return false
	}
	if !IsAgentDID(targetDID) {
		return false
	}
	return PrincipalFromAgentDID(targetDID) == controllerDID
}
