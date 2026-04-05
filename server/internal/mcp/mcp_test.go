package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer() *Server {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewServer(logger)
}

func sendRequest(t *testing.T, s *Server, method string, id int, params any) Response {
	t.Helper()
	req := Request{
		JSONRPC: "2.0",
		Method:  method,
		ID:      json.RawMessage(fmt.Sprintf("%d", id)),
	}
	if params != nil {
		data, _ := json.Marshal(params)
		req.Params = data
	}

	input, _ := json.Marshal(req)
	input = append(input, '\n')

	pr, pw := io.Pipe()
	s.SetIO(bytes.NewReader(input), pw)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = s.Run(ctx)
		pw.Close()
	}()

	// Read the first complete JSON response line from the pipe (blocks until available).
	scanner := bufio.NewScanner(pr)
	require.True(t, scanner.Scan(), "expected a response line from MCP server")
	cancel()

	var resp Response
	require.NoError(t, json.Unmarshal(scanner.Bytes(), &resp))
	return resp
}

// ─── Server Core Tests ──────────────────────────────────────────────────────

func TestServer_Initialize(t *testing.T) {
	s := newTestServer()
	s.RegisterTool(Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]any{"type": "object"},
	}, func(_ context.Context, _ json.RawMessage) (*ToolResult, error) {
		return &ToolResult{Content: []ToolContent{{Type: "text", Text: "ok"}}}, nil
	})

	resp := sendRequest(t, s, "initialize", 1, nil)
	assert.Nil(t, resp.Error)

	data, _ := json.Marshal(resp.Result)
	var result InitializeResult
	require.NoError(t, json.Unmarshal(data, &result))

	assert.Equal(t, "2024-11-05", result.ProtocolVersion)
	assert.Equal(t, "aicp-mcp-server", result.ServerInfo.Name)
	assert.NotNil(t, result.Capabilities.Tools)
}

func TestServer_ToolsList(t *testing.T) {
	s := newTestServer()
	s.RegisterTool(Tool{
		Name:        "tool_a",
		Description: "Tool A",
		InputSchema: map[string]any{"type": "object"},
	}, func(_ context.Context, _ json.RawMessage) (*ToolResult, error) {
		return &ToolResult{Content: []ToolContent{{Type: "text", Text: "a"}}}, nil
	})
	s.RegisterTool(Tool{
		Name:        "tool_b",
		Description: "Tool B",
		InputSchema: map[string]any{"type": "object"},
	}, func(_ context.Context, _ json.RawMessage) (*ToolResult, error) {
		return &ToolResult{Content: []ToolContent{{Type: "text", Text: "b"}}}, nil
	})

	resp := sendRequest(t, s, "tools/list", 2, nil)
	assert.Nil(t, resp.Error)

	data, _ := json.Marshal(resp.Result)
	var result map[string][]Tool
	require.NoError(t, json.Unmarshal(data, &result))
	assert.Len(t, result["tools"], 2)
}

func TestServer_ToolsCall(t *testing.T) {
	s := newTestServer()
	s.RegisterTool(Tool{
		Name:        "echo",
		Description: "Echo tool",
		InputSchema: map[string]any{"type": "object"},
	}, func(_ context.Context, params json.RawMessage) (*ToolResult, error) {
		return &ToolResult{Content: []ToolContent{{Type: "text", Text: string(params)}}}, nil
	})

	resp := sendRequest(t, s, "tools/call", 3, map[string]any{
		"name":      "echo",
		"arguments": map[string]string{"msg": "hello"},
	})
	assert.Nil(t, resp.Error)

	data, _ := json.Marshal(resp.Result)
	var result ToolResult
	require.NoError(t, json.Unmarshal(data, &result))
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "hello")
}

func TestServer_ToolsCall_UnknownTool(t *testing.T) {
	s := newTestServer()

	resp := sendRequest(t, s, "tools/call", 4, map[string]any{
		"name":      "nonexistent",
		"arguments": map[string]string{},
	})
	assert.Nil(t, resp.Error) // Not an RPC error, but tool result is error.

	data, _ := json.Marshal(resp.Result)
	var result ToolResult
	require.NoError(t, json.Unmarshal(data, &result))
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "unknown tool")
}

func TestServer_MethodNotFound(t *testing.T) {
	s := newTestServer()

	resp := sendRequest(t, s, "nonexistent/method", 5, nil)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, CodeMethodNotFound, resp.Error.Code)
}

func TestServer_Ping(t *testing.T) {
	s := newTestServer()

	resp := sendRequest(t, s, "ping", 6, nil)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

func TestServer_ResourcesList(t *testing.T) {
	s := newTestServer()
	s.RegisterResources([]Resource{
		{URI: "aicp://did/test", Name: "Test DID", MimeType: "application/json"},
	}, nil)

	resp := sendRequest(t, s, "resources/list", 7, nil)
	assert.Nil(t, resp.Error)

	data, _ := json.Marshal(resp.Result)
	var result map[string][]Resource
	require.NoError(t, json.Unmarshal(data, &result))
	assert.Len(t, result["resources"], 1)
}

func TestServer_ResourcesRead(t *testing.T) {
	s := newTestServer()
	s.RegisterResources(
		[]Resource{{URI: "aicp://test", Name: "Test"}},
		func(_ context.Context, uri string) ([]ResourceContent, error) {
			return []ResourceContent{{URI: uri, Text: "content for " + uri}}, nil
		},
	)

	resp := sendRequest(t, s, "resources/read", 8, map[string]any{
		"uri": "aicp://test",
	})
	assert.Nil(t, resp.Error)

	data, _ := json.Marshal(resp.Result)
	var result map[string][]ResourceContent
	require.NoError(t, json.Unmarshal(data, &result))
	assert.Contains(t, result["contents"][0].Text, "content for aicp://test")
}

func TestServer_ParseError(t *testing.T) {
	s := newTestServer()

	pr, pw := io.Pipe()
	s.SetIO(strings.NewReader("invalid json\n"), pw)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = s.Run(ctx)
		pw.Close()
	}()

	scanner := bufio.NewScanner(pr)
	require.True(t, scanner.Scan(), "expected error response")
	cancel()

	var resp Response
	require.NoError(t, json.Unmarshal(scanner.Bytes(), &resp))
	assert.NotNil(t, resp.Error)
	assert.Equal(t, CodeParseError, resp.Error.Code)
}

// ─── AICPToolRegistry Tests ─────────────────────────────────────────────────

func TestAICPToolRegistry_RegisterAll(t *testing.T) {
	s := newTestServer()
	registry := NewAICPToolRegistry("http://localhost:8080")
	registry.RegisterAll(s)

	resp := sendRequest(t, s, "tools/list", 10, nil)
	assert.Nil(t, resp.Error)

	data, _ := json.Marshal(resp.Result)
	var result map[string][]Tool
	require.NoError(t, json.Unmarshal(data, &result))

	tools := result["tools"]
	assert.GreaterOrEqual(t, len(tools), 15, "should have at least 15 AICP tools")

	// Verify key tools are registered.
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}
	assert.True(t, names["aicp_did_create"])
	assert.True(t, names["aicp_did_resolve"])
	assert.True(t, names["aicp_discover_search"])
	assert.True(t, names["aicp_onboard_initiate"])
	assert.True(t, names["aicp_payment_request_token"])
	assert.True(t, names["aicp_verify"])
	assert.True(t, names["aicp_trust_score"])
}

func TestAICPToolRegistry_DIDResolve_Integration(t *testing.T) {
	// Mock AICP server.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/aicp/v1/did/did:aicp:test123", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"didDocument": map[string]any{
					"id":         "did:aicp:test123",
					"controller": "did:aicp:test123",
				},
			},
		})
	}))
	defer mockServer.Close()

	s := newTestServer()
	registry := NewAICPToolRegistry(mockServer.URL)
	registry.RegisterAll(s)

	resp := sendRequest(t, s, "tools/call", 11, map[string]any{
		"name":      "aicp_did_resolve",
		"arguments": map[string]string{"did": "did:aicp:test123"},
	})
	assert.Nil(t, resp.Error)

	data, _ := json.Marshal(resp.Result)
	var result ToolResult
	require.NoError(t, json.Unmarshal(data, &result))
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "did:aicp:test123")
}

func TestAICPToolRegistry_DiscoverSearch_Integration(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "capability=payment")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"items": []any{}},
		})
	}))
	defer mockServer.Close()

	s := newTestServer()
	registry := NewAICPToolRegistry(mockServer.URL)
	registry.RegisterAll(s)

	resp := sendRequest(t, s, "tools/call", 12, map[string]any{
		"name":      "aicp_discover_search",
		"arguments": map[string]string{"query": "payment"},
	})
	assert.Nil(t, resp.Error)
}

func TestAICPToolRegistry_OnboardInitiate_Integration(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/aicp/v1/onboard", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"session_id": "sess-123",
			"state":      map[string]any{"phase": "initiated"},
		})
	}))
	defer mockServer.Close()

	s := newTestServer()
	registry := NewAICPToolRegistry(mockServer.URL)
	registry.RegisterAll(s)

	resp := sendRequest(t, s, "tools/call", 13, map[string]any{
		"name": "aicp_onboard_initiate",
		"arguments": map[string]any{
			"agent_did":              "did:aicp:agent1",
			"requested_capabilities": []string{"payment:read"},
		},
	})
	assert.Nil(t, resp.Error)

	data, _ := json.Marshal(resp.Result)
	var result ToolResult
	require.NoError(t, json.Unmarshal(data, &result))
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "sess-123")
}

func TestAICPToolRegistry_ErrorResponse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "INTERNAL_ERROR", "message": "something broke"},
		})
	}))
	defer mockServer.Close()

	s := newTestServer()
	registry := NewAICPToolRegistry(mockServer.URL)
	registry.RegisterAll(s)

	resp := sendRequest(t, s, "tools/call", 14, map[string]any{
		"name":      "aicp_did_resolve",
		"arguments": map[string]string{"did": "did:aicp:fail"},
	})
	assert.Nil(t, resp.Error)

	data, _ := json.Marshal(resp.Result)
	var result ToolResult
	require.NoError(t, json.Unmarshal(data, &result))
	assert.True(t, result.IsError)
}

func TestAICPToolRegistry_MissingRequiredParam(t *testing.T) {
	s := newTestServer()
	registry := NewAICPToolRegistry("http://localhost:8080")
	registry.RegisterAll(s)

	resp := sendRequest(t, s, "tools/call", 15, map[string]any{
		"name":      "aicp_did_resolve",
		"arguments": map[string]string{},
	})
	assert.Nil(t, resp.Error)

	data, _ := json.Marshal(resp.Result)
	var result ToolResult
	require.NoError(t, json.Unmarshal(data, &result))
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "did is required")
}

// ─── Helper Tests ───────────────────────────────────────────────────────────

func TestTextError(t *testing.T) {
	result := textError("something went wrong: %s", "details")
	assert.True(t, result.IsError)
	assert.Equal(t, "something went wrong: details", result.Content[0].Text)
}
