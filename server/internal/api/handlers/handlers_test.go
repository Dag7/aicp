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

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// ─── Mock LedgerBackend ─────────────────────────────────────────────────────

type mockLedgerBackend struct {
	healthy bool
}

func (m *mockLedgerBackend) Healthy(_ context.Context) error {
	if !m.healthy {
		return fmt.Errorf("database unavailable")
	}
	return nil
}

// Remaining stubs.
func (m *mockLedgerBackend) Initialize(context.Context) error                 { return nil }
func (m *mockLedgerBackend) Close() error                                     { return nil }
func (m *mockLedgerBackend) AnchorDID(context.Context, string, aicp.DIDDocument, aicp.Proof) (*aicp.AnchorReceipt, error) {
	return nil, nil
}
func (m *mockLedgerBackend) ResolveDID(context.Context, string) (*aicp.DIDResolutionResult, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) UpdateDIDDocument(context.Context, string, aicp.DIDDocument, aicp.Proof) (*aicp.AnchorReceipt, error) {
	return nil, nil
}
func (m *mockLedgerBackend) DeactivateDID(context.Context, string, aicp.Proof) error { return nil }
func (m *mockLedgerBackend) ListAgentDIDs(context.Context, string) ([]string, error) { return nil, nil }
func (m *mockLedgerBackend) StoreCredential(context.Context, aicp.VerifiableCredential) error {
	return nil
}
func (m *mockLedgerBackend) GetCredential(context.Context, string) (*aicp.VerifiableCredential, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) RevokeCredential(context.Context, string, aicp.Proof) error { return nil }
func (m *mockLedgerBackend) IsCredentialRevoked(context.Context, string) (bool, error) {
	return false, nil
}
func (m *mockLedgerBackend) GetRevocationList(context.Context, string) (*aicp.RevocationList, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) CreateRevocationList(context.Context, aicp.RevocationList) error {
	return nil
}
func (m *mockLedgerBackend) UpdateRevocationList(context.Context, aicp.RevocationList) error {
	return nil
}
func (m *mockLedgerBackend) StoreReceipt(context.Context, aicp.SignedPaymentReceipt) error {
	return nil
}
func (m *mockLedgerBackend) GetReceipt(context.Context, string) (*aicp.SignedPaymentReceipt, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) ListReceiptsByAgent(context.Context, string, aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.SignedPaymentReceipt], error) {
	return &aicp.PaginatedResult[aicp.SignedPaymentReceipt]{}, nil
}
func (m *mockLedgerBackend) IsTokenUsed(context.Context, string) (bool, error)  { return false, nil }
func (m *mockLedgerBackend) MarkTokenUsed(context.Context, string) error         { return nil }
func (m *mockLedgerBackend) AppendEntry(context.Context, aicp.LedgerEntry) error { return nil }
func (m *mockLedgerBackend) GetPendingEntries(context.Context) ([]aicp.LedgerEntry, error) {
	return nil, nil
}
func (m *mockLedgerBackend) StoreBlock(context.Context, aicp.Block) error { return nil }
func (m *mockLedgerBackend) GetLatestBlock(context.Context) (*aicp.Block, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) GetBlock(context.Context, uint64) (*aicp.Block, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) GetTrustScore(context.Context, string) (*aicp.TrustScore, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) UpdateTrustScore(context.Context, aicp.TrustScore) error { return nil }
func (m *mockLedgerBackend) StoreANSRecord(context.Context, aicp.ANSRecord) error    { return nil }
func (m *mockLedgerBackend) GetANSRecord(context.Context, string) (*aicp.ANSRecord, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) SearchANSRecords(context.Context, string, aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.ANSRecord], error) {
	return &aicp.PaginatedResult[aicp.ANSRecord]{}, nil
}
func (m *mockLedgerBackend) DeleteANSRecord(context.Context, string) error         { return nil }
func (m *mockLedgerBackend) RegisterValidator(context.Context, aicp.Validator) error { return nil }
func (m *mockLedgerBackend) RemoveValidator(context.Context, string) error           { return nil }
func (m *mockLedgerBackend) ListValidators(context.Context) ([]aicp.Validator, error) {
	return nil, nil
}
func (m *mockLedgerBackend) GetValidator(context.Context, string) (*aicp.Validator, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) GetDailySpending(context.Context, string, string) (string, error) {
	return "0", nil
}
func (m *mockLedgerBackend) GetMonthlySpending(context.Context, string, string) (string, error) {
	return "0", nil
}
func (m *mockLedgerBackend) RecordSpending(context.Context, string, aicp.MoneyAmount) error {
	return nil
}

// ─── Health Handler Tests ────────────────────────────────────────────────────

func TestHealthHandler_Health(t *testing.T) {
	handler := NewHealthHandler(&mockLedgerBackend{healthy: true})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.Health(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ok", data["status"])
	assert.Equal(t, aicp.ProtocolName, data["service"])
	assert.Equal(t, aicp.ProtocolVersion, data["version"])
}

func TestHealthHandler_Health_AlwaysReturns200(t *testing.T) {
	// Health is a liveness probe -- always returns 200 regardless of backend state.
	handler := NewHealthHandler(&mockLedgerBackend{healthy: false})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.Health(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthHandler_Ready_Healthy(t *testing.T) {
	handler := NewHealthHandler(&mockLedgerBackend{healthy: true})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handler.Ready(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ready", data["status"])
}

func TestHealthHandler_Ready_Unhealthy(t *testing.T) {
	handler := NewHealthHandler(&mockLedgerBackend{healthy: false})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handler.Ready(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "NOT_READY", resp.Error.Code)
}

// ─── Response Helper Tests ───────────────────────────────────────────────────

func TestJSON_WritesSuccessEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	JSON(w, http.StatusOK, map[string]string{"key": "value"})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Nil(t, resp.Error)
}

func TestJSONError_WritesErrorEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	JSONError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid input")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "BAD_REQUEST", resp.Error.Code)
	assert.Equal(t, "invalid input", resp.Error.Message)
}

func TestJSONErrorDetail_IncludesDetail(t *testing.T) {
	w := httptest.NewRecorder()
	JSONErrorDetail(w, http.StatusNotFound, "NOT_FOUND", "resource not found", "DID did:aicp:abc not found")

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "NOT_FOUND", resp.Error.Code)
	assert.Equal(t, "DID did:aicp:abc not found", resp.Error.Detail)
}

func TestProtocolErrorResponse_ProtocolError(t *testing.T) {
	w := httptest.NewRecorder()
	pe := aicp.NewProtocolError(http.StatusNotFound, "DID_NOT_FOUND", "DID not found")
	ProtocolErrorResponse(w, pe)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "DID_NOT_FOUND", resp.Error.Code)
}

func TestProtocolErrorResponse_GenericError(t *testing.T) {
	w := httptest.NewRecorder()
	ProtocolErrorResponse(w, fmt.Errorf("some internal error"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "INTERNAL_ERROR", resp.Error.Code)
}

func TestDecodeJSON(t *testing.T) {
	body := strings.NewReader(`{"agent_did":"did:aicp:agent","service_did":"did:aicp:svc"}`)
	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", "application/json")

	var dest aicp.OnboardInitRequest
	err := DecodeJSON(req, &dest)
	require.NoError(t, err)
	assert.Equal(t, "did:aicp:agent", dest.AgentDID)
}

func TestDecodeJSON_InvalidBody(t *testing.T) {
	body := strings.NewReader("not json")
	req := httptest.NewRequest(http.MethodPost, "/test", body)

	var dest aicp.OnboardInitRequest
	err := DecodeJSON(req, &dest)
	require.Error(t, err)
}
