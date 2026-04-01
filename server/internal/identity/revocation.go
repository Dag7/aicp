package identity

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

const (
	defaultListCapacity = 131072 // 128K credentials per list.
)

// RevocationManager manages RevocationList2020 bitstrings for credential revocation.
type RevocationManager struct {
	backend ledger.LedgerBackend
}

// NewRevocationManager creates a new revocation manager.
func NewRevocationManager(backend ledger.LedgerBackend) *RevocationManager {
	return &RevocationManager{backend: backend}
}

// CreateList creates a new revocation list for an issuer.
func (rm *RevocationManager) CreateList(ctx context.Context, issuerDID string) (*aicp.RevocationList, error) {
	listID := aicp.RevocationIDPrefix + crypto.SHA256Hex([]byte(issuerDID + ":revocation-list"))[:16]

	// Create an empty bitstring (all zeros).
	bitstring := make([]byte, defaultListCapacity/8)
	encoded, err := encodeBitstring(bitstring)
	if err != nil {
		return nil, fmt.Errorf("encoding bitstring: %w", err)
	}

	list := aicp.RevocationList{
		ListID:        listID,
		IssuerDID:     issuerDID,
		EncodedList:   encoded,
		TotalCapacity: defaultListCapacity,
		NextFreeIndex: 0,
		VersionID:     1,
	}

	if err := rm.backend.CreateRevocationList(ctx, list); err != nil {
		return nil, fmt.Errorf("creating revocation list: %w", err)
	}

	return &list, nil
}

// AllocateIndex allocates the next free index in a revocation list.
// Returns the allocated index.
func (rm *RevocationManager) AllocateIndex(ctx context.Context, listID string) (int, error) {
	list, err := rm.backend.GetRevocationList(ctx, listID)
	if err != nil {
		return 0, fmt.Errorf("getting revocation list: %w", err)
	}

	if list.NextFreeIndex >= list.TotalCapacity {
		return 0, fmt.Errorf("revocation list %s is full", listID)
	}

	idx := list.NextFreeIndex
	list.NextFreeIndex = idx + 1

	if err := rm.backend.UpdateRevocationList(ctx, *list); err != nil {
		return 0, fmt.Errorf("updating revocation list: %w", err)
	}

	return idx, nil
}

// Revoke sets the bit at the given index in the revocation list, marking
// the credential at that index as revoked.
func (rm *RevocationManager) Revoke(ctx context.Context, listID string, index int) error {
	list, err := rm.backend.GetRevocationList(ctx, listID)
	if err != nil {
		return fmt.Errorf("getting revocation list: %w", err)
	}

	bitstring, err := decodeBitstring(list.EncodedList)
	if err != nil {
		return fmt.Errorf("decoding bitstring: %w", err)
	}

	if index < 0 || index >= list.TotalCapacity {
		return fmt.Errorf("index %d out of range [0, %d)", index, list.TotalCapacity)
	}

	// Set the bit.
	byteIdx := index / 8
	bitIdx := uint(index % 8)
	bitstring[byteIdx] |= 1 << bitIdx

	encoded, err := encodeBitstring(bitstring)
	if err != nil {
		return fmt.Errorf("encoding bitstring: %w", err)
	}

	list.EncodedList = encoded
	return rm.backend.UpdateRevocationList(ctx, *list)
}

// IsRevoked checks whether the credential at the given index is revoked.
func (rm *RevocationManager) IsRevoked(ctx context.Context, listID string, index int) (bool, error) {
	list, err := rm.backend.GetRevocationList(ctx, listID)
	if err != nil {
		return false, fmt.Errorf("getting revocation list: %w", err)
	}

	bitstring, err := decodeBitstring(list.EncodedList)
	if err != nil {
		return false, fmt.Errorf("decoding bitstring: %w", err)
	}

	if index < 0 || index >= list.TotalCapacity {
		return false, fmt.Errorf("index %d out of range [0, %d)", index, list.TotalCapacity)
	}

	byteIdx := index / 8
	bitIdx := uint(index % 8)
	return (bitstring[byteIdx] & (1 << bitIdx)) != 0, nil
}

// GetList returns the revocation list.
func (rm *RevocationManager) GetList(ctx context.Context, listID string) (*aicp.RevocationList, error) {
	return rm.backend.GetRevocationList(ctx, listID)
}

// encodeBitstring GZIP-compresses and base64-encodes a bitstring.
func encodeBitstring(bitstring []byte) (string, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(bitstring); err != nil {
		return "", fmt.Errorf("gzip write: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("gzip close: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// decodeBitstring base64-decodes and GZIP-decompresses a bitstring.
func decodeBitstring(encoded string) ([]byte, error) {
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	r, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = r.Close() }()
	return io.ReadAll(r)
}
