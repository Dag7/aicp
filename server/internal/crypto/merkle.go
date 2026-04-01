package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

// Domain separation prefixes per RFC 6962 to prevent second preimage attacks.
// Leaf hashes are prefixed with 0x00, internal (branch) hashes with 0x01.
var (
	leafPrefix   = []byte{0x00}
	branchPrefix = []byte{0x01}
)

// MerkleTree computes a Merkle root from a list of leaf hashes.
// Each leaf hash should be a hex-encoded SHA-256 hash.
// Returns the hex-encoded Merkle root.
func MerkleRoot(leafHashes []string) (string, error) {
	if len(leafHashes) == 0 {
		return SHA256Hex([]byte("empty")), nil
	}

	nodes := make([][]byte, len(leafHashes))
	for i, h := range leafHashes {
		decoded, err := hex.DecodeString(h)
		if err != nil {
			return "", err
		}
		// Prefix leaf data with 0x00 before hashing (RFC 6962).
		leafData := append(leafPrefix, decoded...)
		hash := sha256.Sum256(leafData)
		nodes[i] = hash[:]
	}

	for len(nodes) > 1 {
		var next [][]byte
		for i := 0; i < len(nodes); i += 2 {
			if i+1 < len(nodes) {
				// Prefix internal node concatenation with 0x01 (RFC 6962).
				combined := append(branchPrefix, nodes[i]...)
				combined = append(combined, nodes[i+1]...)
				hash := sha256.Sum256(combined)
				next = append(next, hash[:])
			} else {
				// Odd node: hash with itself, still using branch prefix.
				combined := append(branchPrefix, nodes[i]...)
				combined = append(combined, nodes[i]...)
				hash := sha256.Sum256(combined)
				next = append(next, hash[:])
			}
		}
		nodes = next
	}

	return hex.EncodeToString(nodes[0]), nil
}

// MerkleVerifyProof verifies a Merkle inclusion proof for a leaf.
// proof is the list of sibling hashes from leaf to root.
// index is the leaf's position (0-based) in the original list.
func MerkleVerifyProof(leafHash string, proof []string, index int, root string) (bool, error) {
	decoded, err := hex.DecodeString(leafHash)
	if err != nil {
		return false, err
	}

	// Hash the leaf with 0x00 prefix, matching MerkleRoot construction.
	leafData := append(leafPrefix, decoded...)
	leafH := sha256.Sum256(leafData)
	current := leafH[:]

	idx := index
	for _, siblingHex := range proof {
		sibling, err := hex.DecodeString(siblingHex)
		if err != nil {
			return false, err
		}

		// Prefix internal node concatenation with 0x01 (RFC 6962).
		var combined []byte
		if idx%2 == 0 {
			combined = append(branchPrefix, current...)
			combined = append(combined, sibling...)
		} else {
			combined = append(branchPrefix, sibling...)
			combined = append(combined, current...)
		}
		hash := sha256.Sum256(combined)
		current = hash[:]
		idx /= 2
	}

	return hex.EncodeToString(current) == root, nil
}
