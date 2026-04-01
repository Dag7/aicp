// Package mcp implements a Model Context Protocol (MCP) server that exposes
// AICP protocol operations as MCP tools. The server uses JSON-RPC 2.0 over
// stdio transport, enabling AI agents and LLMs to interact with AICP services.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

// ─── JSON-RPC 2.0 Types ────────────────────────────────────────────────────

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Notification is a JSON-RPC 2.0 notification (no id, no response expected).
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Standard JSON-RPC error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// ─── MCP Protocol Types ────────────────────────────────────────────────────

// ServerCapabilities declares what the MCP server supports.
type ServerCapabilities struct {
	Tools     *ToolCapability     `json:"tools,omitempty"`
	Resources *ResourceCapability `json:"resources,omitempty"`
}

// ToolCapability declares tool support.
type ToolCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourceCapability declares resource support.
type ResourceCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerInfo describes the MCP server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is returned from the initialize method.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// Tool describes an MCP tool.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema"`
}

// ToolResult is returned from a tool call.
type ToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ToolContent is a piece of content in a tool result.
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Resource describes an MCP resource.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceContent is a piece of resource content.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

// ─── MCP Server ────────────────────────────────────────────────────────────

// ToolHandler handles an MCP tool call and returns the result.
type ToolHandler func(ctx context.Context, params json.RawMessage) (*ToolResult, error)

// ResourceHandler handles an MCP resource read.
type ResourceHandler func(ctx context.Context, uri string) ([]ResourceContent, error)

// Server implements an MCP server with JSON-RPC 2.0 over stdio.
type Server struct {
	mu        sync.RWMutex
	tools     []Tool
	handlers  map[string]ToolHandler
	resources []Resource
	resReader ResourceHandler
	logger    *slog.Logger
	in        io.Reader
	out       io.Writer
}

// NewServer creates a new MCP server.
func NewServer(logger *slog.Logger) *Server {
	return &Server{
		handlers: make(map[string]ToolHandler),
		logger:   logger,
		in:       os.Stdin,
		out:      os.Stdout,
	}
}

// SetIO sets custom input/output streams (useful for testing).
// Must be called before Run.
func (s *Server) SetIO(in io.Reader, out io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.in = in
	s.out = out
}

// RegisterTool adds a tool to the server.
func (s *Server) RegisterTool(tool Tool, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools = append(s.tools, tool)
	s.handlers[tool.Name] = handler
}

// RegisterResources sets the resource list and reader.
func (s *Server) RegisterResources(resources []Resource, reader ResourceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources = resources
	s.resReader = reader
}

// Run starts the MCP server, reading JSON-RPC messages from stdin and writing
// responses to stdout. It blocks until the context is cancelled or stdin closes.
func (s *Server) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(s.in)
	// MCP uses newline-delimited JSON.
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max message

	s.logger.Info("MCP server started", "transport", "stdio")

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeResponse(Response{
				JSONRPC: "2.0",
				Error:   &RPCError{Code: CodeParseError, Message: "parse error"},
			})
			continue
		}

		// Notifications (no id) don't get a response per JSON-RPC 2.0.
		if req.ID == nil {
			s.logger.Debug("notification received", "method", req.Method)
			continue
		}

		resp := s.handleRequest(ctx, req)
		s.writeResponse(resp)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}

	return nil
}

func (s *Server) handleRequest(ctx context.Context, req Request) Response {
	resp := Response{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case "initialize":
		resp.Result = s.handleInitialize()
	case "tools/list":
		resp.Result = s.handleToolsList()
	case "tools/call":
		result, err := s.handleToolsCall(ctx, req.Params)
		if err != nil {
			s.logger.Error("tools/call failed", "error", err)
			resp.Error = &RPCError{Code: CodeInvalidParams, Message: "invalid tool call parameters"}
		} else {
			resp.Result = result
		}
	case "resources/list":
		resp.Result = s.handleResourcesList()
	case "resources/read":
		result, err := s.handleResourcesRead(ctx, req.Params)
		if err != nil {
			s.logger.Error("resources/read failed", "error", err)
			resp.Error = &RPCError{Code: CodeInternalError, Message: "resource read failed"}
		} else {
			resp.Result = result
		}
	case "ping":
		resp.Result = map[string]any{}
	default:
		resp.Error = &RPCError{Code: CodeMethodNotFound, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}

	return resp
}

func (s *Server) handleInitialize() InitializeResult {
	caps := ServerCapabilities{
		Tools: &ToolCapability{},
	}
	s.mu.RLock()
	if len(s.resources) > 0 {
		caps.Resources = &ResourceCapability{}
	}
	s.mu.RUnlock()

	return InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    caps,
		ServerInfo: ServerInfo{
			Name:    "aicp-mcp-server",
			Version: "1.0.0",
		},
	}
}

func (s *Server) handleToolsList() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]any{"tools": s.tools}
}

func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, fmt.Errorf("invalid tool call params: %w", err)
	}

	s.mu.RLock()
	handler, ok := s.handlers[call.Name]
	s.mu.RUnlock()

	if !ok {
		return &ToolResult{
			Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("unknown tool: %s", call.Name)}},
			IsError: true,
		}, nil
	}

	s.logger.Debug("executing tool", "tool", call.Name)
	result, err := handler(ctx, call.Arguments)
	if err != nil {
		return &ToolResult{
			Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("tool error: %s", err.Error())}},
			IsError: true,
		}, nil
	}

	return result, nil
}

func (s *Server) handleResourcesList() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]any{"resources": s.resources}
}

func (s *Server) handleResourcesRead(ctx context.Context, params json.RawMessage) (map[string]any, error) {
	var read struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &read); err != nil {
		return nil, fmt.Errorf("invalid resource read params: %w", err)
	}

	s.mu.RLock()
	reader := s.resReader
	s.mu.RUnlock()

	if reader == nil {
		return nil, fmt.Errorf("no resource reader configured")
	}

	contents, err := reader(ctx, read.URI)
	if err != nil {
		return nil, err
	}

	return map[string]any{"contents": contents}, nil
}

func (s *Server) writeResponse(resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		s.logger.Error("failed to marshal response", "error", err)
		return
	}
	fmt.Fprintf(s.out, "%s\n", data)
}
