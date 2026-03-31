package crypto

import (
	"crypto/sha256"
	"encoding/hex"
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
		nodes[i] = decoded
	}

	for len(nodes) > 1 {
		var next [][]byte
		for i := 0; i < len(nodes); i += 2 {
			if i+1 < len(nodes) {
				combined := append(nodes[i], nodes[i+1]...)
				hash := sha256.Sum256(combined)
				next = append(next, hash[:])
			} else {
				// Odd node: hash with itself.
				combined := append(nodes[i], nodes[i]...)
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
	current, err := hex.DecodeString(leafHash)
	if err != nil {
		return false, err
	}

	idx := index
	for _, siblingHex := range proof {
		sibling, err := hex.DecodeString(siblingHex)
		if err != nil {
			return false, err
		}

		var combined []byte
		if idx%2 == 0 {
			combined = append(current, sibling...)
		} else {
			combined = append(sibling, current...)
		}
		hash := sha256.Sum256(combined)
		current = hash[:]
		idx /= 2
	}

	return hex.EncodeToString(current) == root, nil
}
