// Copyright 2024 AICP Protocol Contributors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// MemoryBackend is an in-memory implementation of LedgerBackend intended for
// unit tests. All data is stored in maps and slices protected by a RWMutex.
type MemoryBackend struct {
	mu sync.RWMutex

	// DID storage
	didDocs       map[string]aicp.DIDDocument        // did -> latest doc
	didMeta       map[string]*aicp.DIDDocumentMetadata // did -> metadata
	didDeactivated map[string]bool                     // did -> deactivated

	// Credential storage
	credentials       map[string]aicp.VerifiableCredential // credentialID -> VC
	credentialRevoked map[string]bool                      // credentialID -> revoked

	// Revocation lists
	revocationLists map[string]aicp.RevocationList // listID -> list

	// Receipt storage
	receipts  map[string]aicp.SignedPaymentReceipt // receiptID -> receipt
	usedJTIs  map[string]bool                      // jti -> used

	// Ledger entries and blocks
	entries     []aicp.LedgerEntry        // all entries, ordered by ID
	nextEntryID uint64                    // auto-increment for entry IDs
	blocks      map[uint64]aicp.Block     // number -> block

	// Trust scores
	trustScores map[string]aicp.TrustScore // did -> score

	// ANS records
	ansRecords map[string]aicp.ANSRecord // serviceDID -> record

	// Validators
	validators map[string]aicp.Validator // validatorDID -> validator

	// Spending records
	spendingRecords []spendingRecord
}

// spendingRecord tracks a single spending event for limit calculation.
type spendingRecord struct {
	DID        string
	Amount     aicp.MoneyAmount
	RecordedAt time.Time
}

// NewMemoryBackend creates a new in-memory backend with all maps initialized.
func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		didDocs:           make(map[string]aicp.DIDDocument),
		didMeta:           make(map[string]*aicp.DIDDocumentMetadata),
		didDeactivated:    make(map[string]bool),
		credentials:       make(map[string]aicp.VerifiableCredential),
		credentialRevoked: make(map[string]bool),
		revocationLists:   make(map[string]aicp.RevocationList),
		receipts:          make(map[string]aicp.SignedPaymentReceipt),
		usedJTIs:          make(map[string]bool),
		nextEntryID:       1,
		blocks:            make(map[uint64]aicp.Block),
		trustScores:       make(map[string]aicp.TrustScore),
		ansRecords:        make(map[string]aicp.ANSRecord),
		validators:        make(map[string]aicp.Validator),
	}
}

// Compile-time interface satisfaction check.
var _ LedgerBackend = (*MemoryBackend)(nil)

// ── Lifecycle ────────────────────────────────────────────────────────────────

func (m *MemoryBackend) Initialize(_ context.Context) error { return nil }
func (m *MemoryBackend) Close() error                       { return nil }
func (m *MemoryBackend) Healthy(_ context.Context) error     { return nil }

// ── DID Operations ───────────────────────────────────────────────────────────

func (m *MemoryBackend) AnchorDID(_ context.Context, did string, doc aicp.DIDDocument, _ aicp.Proof) (*aicp.AnchorReceipt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.didDocs[did]; exists {
		return nil, aicp.ErrAlreadyExists
	}

	docJSON, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshaling DID document: %w", err)
	}
	docHash := crypto.SHA256Hex(docJSON)
	now := time.Now().UTC()

	m.didDocs[did] = doc
	m.didMeta[did] = &aicp.DIDDocumentMetadata{
		Created:      now,
		Updated:      now,
		VersionID:    1,
		Deactivated:  false,
		DocumentHash: docHash,
	}

	entryHash := crypto.SHA256Hex([]byte(did + ":create:" + docHash))
	m.appendEntryLocked(aicp.LedgerEntry{
		EntryType:    "did_create",
		EntryHash:    entryHash,
		EntryDataRef: did,
		RecordedAt:   now,
	})

	return &aicp.AnchorReceipt{
		DID:          did,
		EntryHash:    entryHash,
		DocumentHash: docHash,
		AnchoredAt:   now,
	}, nil
}

func (m *MemoryBackend) ResolveDID(_ context.Context, did string) (*aicp.DIDResolutionResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	doc, exists := m.didDocs[did]
	if !exists {
		return nil, aicp.ErrNotFound
	}

	meta := m.didMeta[did]
	return &aicp.DIDResolutionResult{
		Document:         &doc,
		DocumentMetadata: meta,
	}, nil
}

func (m *MemoryBackend) UpdateDIDDocument(_ context.Context, did string, doc aicp.DIDDocument, _ aicp.Proof) (*aicp.AnchorReceipt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.didDocs[did]; !exists {
		return nil, aicp.ErrNotFound
	}
	if m.didDeactivated[did] {
		return nil, aicp.ErrNotFound
	}

	docJSON, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshaling DID document: %w", err)
	}
	docHash := crypto.SHA256Hex(docJSON)
	now := time.Now().UTC()

	m.didDocs[did] = doc
	meta := m.didMeta[did]
	meta.Updated = now
	meta.VersionID++
	meta.DocumentHash = docHash

	entryHash := crypto.SHA256Hex([]byte(did + ":update:" + docHash))
	m.appendEntryLocked(aicp.LedgerEntry{
		EntryType:    "did_update",
		EntryHash:    entryHash,
		EntryDataRef: did,
		RecordedAt:   now,
	})

	return &aicp.AnchorReceipt{
		DID:          did,
		EntryHash:    entryHash,
		DocumentHash: docHash,
		AnchoredAt:   now,
	}, nil
}

func (m *MemoryBackend) DeactivateDID(_ context.Context, did string, _ aicp.Proof) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.didDocs[did]; !exists {
		return aicp.ErrNotFound
	}
	if m.didDeactivated[did] {
		return aicp.ErrNotFound
	}

	m.didDeactivated[did] = true
	meta := m.didMeta[did]
	meta.Deactivated = true
	meta.Updated = time.Now().UTC()

	entryHash := crypto.SHA256Hex([]byte(did + ":deactivate"))
	m.appendEntryLocked(aicp.LedgerEntry{
		EntryType:    "did_deactivate",
		EntryHash:    entryHash,
		EntryDataRef: did,
		RecordedAt:   time.Now().UTC(),
	})

	return nil
}

func (m *MemoryBackend) ListAgentDIDs(_ context.Context, principalDID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var agents []string
	for did, doc := range m.didDocs {
		if doc.Controller == principalDID && did != principalDID && !m.didDeactivated[did] {
			agents = append(agents, did)
		}
	}
	return agents, nil
}

// ── Credential Operations ────────────────────────────────────────────────────

func (m *MemoryBackend) StoreCredential(_ context.Context, cred aicp.VerifiableCredential) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.credentials[cred.ID] = cred
	return nil
}

func (m *MemoryBackend) GetCredential(_ context.Context, credentialID string) (*aicp.VerifiableCredential, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cred, exists := m.credentials[credentialID]
	if !exists {
		return nil, aicp.ErrNotFound
	}
	return &cred, nil
}

func (m *MemoryBackend) RevokeCredential(_ context.Context, credentialID string, _ aicp.Proof) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.credentials[credentialID]; !exists {
		return aicp.ErrNotFound
	}
	if m.credentialRevoked[credentialID] {
		return aicp.ErrNotFound
	}

	m.credentialRevoked[credentialID] = true

	entryHash := crypto.SHA256Hex([]byte("credential_revoke:" + credentialID))
	m.appendEntryLocked(aicp.LedgerEntry{
		EntryType:    "credential_revoke",
		EntryHash:    entryHash,
		EntryDataRef: credentialID,
		RecordedAt:   time.Now().UTC(),
	})

	return nil
}

func (m *MemoryBackend) IsCredentialRevoked(_ context.Context, credentialID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.credentials[credentialID]; !exists {
		return false, aicp.ErrNotFound
	}
	return m.credentialRevoked[credentialID], nil
}

func (m *MemoryBackend) GetRevocationList(_ context.Context, listID string) (*aicp.RevocationList, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list, exists := m.revocationLists[listID]
	if !exists {
		return nil, aicp.ErrNotFound
	}
	return &list, nil
}

func (m *MemoryBackend) CreateRevocationList(_ context.Context, list aicp.RevocationList) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.revocationLists[list.ListID]; exists {
		return aicp.ErrAlreadyExists
	}
	list.VersionID = 1
	list.UpdatedAt = time.Now().UTC()
	m.revocationLists[list.ListID] = list
	return nil
}

func (m *MemoryBackend) UpdateRevocationList(_ context.Context, list aicp.RevocationList) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.revocationLists[list.ListID]
	if !exists {
		return aicp.ErrNotFound
	}
	existing.EncodedList = list.EncodedList
	existing.NextFreeIndex = list.NextFreeIndex
	existing.VersionID++
	existing.UpdatedAt = time.Now().UTC()
	m.revocationLists[list.ListID] = existing
	return nil
}

// ── Receipt Operations ───────────────────────────────────────────────────────

func (m *MemoryBackend) StoreReceipt(_ context.Context, receipt aicp.SignedPaymentReceipt) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.receipts[receipt.ReceiptID] = receipt

	entryHash := crypto.SHA256Hex([]byte("receipt:" + receipt.ReceiptID + ":" + receipt.ReceiptHash))
	m.appendEntryLocked(aicp.LedgerEntry{
		EntryType:    "payment_receipt",
		EntryHash:    entryHash,
		EntryDataRef: receipt.ReceiptID,
		RecordedAt:   time.Now().UTC(),
	})

	return nil
}

func (m *MemoryBackend) GetReceipt(_ context.Context, receiptID string) (*aicp.SignedPaymentReceipt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, exists := m.receipts[receiptID]
	if !exists {
		return nil, aicp.ErrNotFound
	}
	return &r, nil
}

func (m *MemoryBackend) ListReceiptsByAgent(_ context.Context, agentDID string, opts aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.SignedPaymentReceipt], error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := 0
	if opts.Cursor != "" {
		parsed, err := strconv.Atoi(opts.Cursor)
		if err == nil {
			offset = parsed
		}
	}

	// Collect matching receipts.
	var matching []aicp.SignedPaymentReceipt
	for _, r := range m.receipts {
		if r.PayerDID == agentDID || r.PayeeDID == agentDID {
			matching = append(matching, r)
		}
	}

	// Apply offset.
	if offset > len(matching) {
		offset = len(matching)
	}
	matching = matching[offset:]

	hasMore := len(matching) > limit
	if hasMore {
		matching = matching[:limit]
	}

	result := &aicp.PaginatedResult[aicp.SignedPaymentReceipt]{
		Items:   matching,
		HasMore: hasMore,
	}
	if hasMore {
		result.NextCursor = strconv.Itoa(offset + limit)
	}
	if result.Items == nil {
		result.Items = []aicp.SignedPaymentReceipt{}
	}
	return result, nil
}

func (m *MemoryBackend) IsTokenUsed(_ context.Context, jti string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.usedJTIs[jti], nil
}

func (m *MemoryBackend) MarkTokenUsed(_ context.Context, jti string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.usedJTIs[jti] = true
	return nil
}

// ── Block / Consensus Operations ─────────────────────────────────────────────

func (m *MemoryBackend) AppendEntry(_ context.Context, entry aicp.LedgerEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.appendEntryLocked(entry)
	return nil
}

// appendEntryLocked adds an entry without acquiring the lock (caller must hold it).
func (m *MemoryBackend) appendEntryLocked(entry aicp.LedgerEntry) {
	entry.ID = m.nextEntryID
	m.nextEntryID++
	if entry.RecordedAt.IsZero() {
		entry.RecordedAt = time.Now().UTC()
	}
	m.entries = append(m.entries, entry)
}

func (m *MemoryBackend) GetPendingEntries(_ context.Context) ([]aicp.LedgerEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var pending []aicp.LedgerEntry
	for _, e := range m.entries {
		if e.BlockNumber == 0 {
			pending = append(pending, e)
		}
	}
	return pending, nil
}

func (m *MemoryBackend) StoreBlock(_ context.Context, block aicp.Block) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.blocks[block.Number] = block

	// Assign the oldest pending entries to this block, matching PostgresBackend behavior.
	assigned := 0
	for i := range m.entries {
		if m.entries[i].BlockNumber == 0 {
			m.entries[i].BlockNumber = block.Number
			assigned++
			if assigned >= block.EntryCount {
				break
			}
		}
	}

	return nil
}

func (m *MemoryBackend) GetLatestBlock(_ context.Context) (*aicp.Block, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.blocks) == 0 {
		return nil, aicp.ErrNotFound
	}

	var latest aicp.Block
	var found bool
	for _, b := range m.blocks {
		if !found || b.Number > latest.Number {
			latest = b
			found = true
		}
	}
	return &latest, nil
}

func (m *MemoryBackend) GetBlock(_ context.Context, number uint64) (*aicp.Block, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	b, exists := m.blocks[number]
	if !exists {
		return nil, aicp.ErrNotFound
	}
	return &b, nil
}

// ── Trust Operations ─────────────────────────────────────────────────────────

func (m *MemoryBackend) GetTrustScore(_ context.Context, did string) (*aicp.TrustScore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ts, exists := m.trustScores[did]
	if !exists {
		return nil, aicp.ErrNotFound
	}
	return &ts, nil
}

func (m *MemoryBackend) UpdateTrustScore(_ context.Context, score aicp.TrustScore) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	score.ComputedAt = time.Now().UTC()
	m.trustScores[score.DID] = score
	return nil
}

// ── ANS Operations ───────────────────────────────────────────────────────────

func (m *MemoryBackend) StoreANSRecord(_ context.Context, record aicp.ANSRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ansRecords[record.Service.DID] = record
	return nil
}

func (m *MemoryBackend) GetANSRecord(_ context.Context, serviceDID string) (*aicp.ANSRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, exists := m.ansRecords[serviceDID]
	if !exists {
		return nil, aicp.ErrNotFound
	}
	return &r, nil
}

func (m *MemoryBackend) SearchANSRecords(_ context.Context, query string, opts aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.ANSRecord], error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := 0
	if opts.Cursor != "" {
		parsed, err := strconv.Atoi(opts.Cursor)
		if err == nil {
			offset = parsed
		}
	}

	queryLower := strings.ToLower(query)

	var matching []aicp.ANSRecord
	for _, r := range m.ansRecords {
		if matchesANSQuery(r, queryLower) {
			matching = append(matching, r)
		}
	}

	// Apply offset.
	if offset > len(matching) {
		offset = len(matching)
	}
	matching = matching[offset:]

	hasMore := len(matching) > limit
	if hasMore {
		matching = matching[:limit]
	}

	result := &aicp.PaginatedResult[aicp.ANSRecord]{
		Items:   matching,
		HasMore: hasMore,
	}
	if hasMore {
		result.NextCursor = strconv.Itoa(offset + limit)
	}
	if result.Items == nil {
		result.Items = []aicp.ANSRecord{}
	}
	return result, nil
}

// matchesANSQuery performs a simple case-insensitive substring match against
// the service name, description, categories, and discovery tags.
func matchesANSQuery(r aicp.ANSRecord, queryLower string) bool {
	if strings.Contains(strings.ToLower(r.Service.Name), queryLower) {
		return true
	}
	if strings.Contains(strings.ToLower(r.Service.Description), queryLower) {
		return true
	}
	for _, cat := range r.Service.Category {
		if strings.Contains(strings.ToLower(cat), queryLower) {
			return true
		}
	}
	for _, tag := range r.DiscoveryTags {
		if strings.Contains(strings.ToLower(tag), queryLower) {
			return true
		}
	}
	return false
}

func (m *MemoryBackend) DeleteANSRecord(_ context.Context, serviceDID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.ansRecords[serviceDID]; !exists {
		return aicp.ErrNotFound
	}
	delete(m.ansRecords, serviceDID)
	return nil
}

// ── Validator Operations ─────────────────────────────────────────────────────

func (m *MemoryBackend) RegisterValidator(_ context.Context, v aicp.Validator) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	v.Active = true
	if v.JoinedAt.IsZero() {
		v.JoinedAt = time.Now().UTC()
	}
	m.validators[v.DID] = v
	return nil
}

func (m *MemoryBackend) RemoveValidator(_ context.Context, validatorDID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	v, exists := m.validators[validatorDID]
	if !exists || !v.Active {
		return aicp.ErrNotFound
	}
	v.Active = false
	m.validators[validatorDID] = v
	return nil
}

func (m *MemoryBackend) ListValidators(_ context.Context) ([]aicp.Validator, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []aicp.Validator
	for _, v := range m.validators {
		if v.Active {
			active = append(active, v)
		}
	}
	return active, nil
}

func (m *MemoryBackend) GetValidator(_ context.Context, validatorDID string) (*aicp.Validator, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	v, exists := m.validators[validatorDID]
	if !exists {
		return nil, aicp.ErrNotFound
	}
	return &v, nil
}

// ── Spending Tracking ────────────────────────────────────────────────────────

func (m *MemoryBackend) GetDailySpending(_ context.Context, did string, currency string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	return m.sumSpending(did, currency, startOfDay), nil
}

func (m *MemoryBackend) GetMonthlySpending(_ context.Context, did string, currency string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now().UTC()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	return m.sumSpending(did, currency, startOfMonth), nil
}

// sumSpending totals spending for a DID+currency from the given time onward.
// Caller must hold at least a read lock.
func (m *MemoryBackend) sumSpending(did, currency string, since time.Time) string {
	var totalCents int64
	for _, sr := range m.spendingRecords {
		if sr.DID == did && sr.Amount.Currency == currency && !sr.RecordedAt.Before(since) {
			val, err := strconv.ParseFloat(sr.Amount.Value, 64)
			if err == nil {
				totalCents += int64(val * 100)
			}
		}
	}
	whole := totalCents / 100
	frac := totalCents % 100
	if frac < 0 {
		frac = -frac
	}
	return fmt.Sprintf("%d.%02d", whole, frac)
}

func (m *MemoryBackend) RecordSpending(_ context.Context, did string, amount aicp.MoneyAmount) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.spendingRecords = append(m.spendingRecords, spendingRecord{
		DID:        did,
		Amount:     amount,
		RecordedAt: time.Now().UTC(),
	})
	return nil
}
