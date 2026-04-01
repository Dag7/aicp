package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AICPToolRegistry registers all AICP protocol tools with an MCP server.
// It acts as a thin adapter between MCP tool calls and the AICP REST API.
type AICPToolRegistry struct {
	serverURL string
	client    *http.Client
}

// NewAICPToolRegistry creates a new tool registry that forwards tool calls
// to the AICP server at the given URL.
func NewAICPToolRegistry(serverURL string) *AICPToolRegistry {
	return &AICPToolRegistry{
		serverURL: strings.TrimRight(serverURL, "/"),
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// RegisterAll registers all AICP tools with the given MCP server.
func (r *AICPToolRegistry) RegisterAll(s *Server) {
	r.registerIdentityTools(s)
	r.registerDiscoveryTools(s)
	r.registerOnboardingTools(s)
	r.registerCommerceTools(s)
	r.registerVerifyTools(s)
}

// maxInputLen is the maximum length for user-supplied string parameters.
const maxInputLen = 512

// ─── Identity Tools ─────────────────────────────────────────────────────────

func (r *AICPToolRegistry) registerIdentityTools(s *Server) {
	s.RegisterTool(Tool{
		Name:        "aicp_did_create",
		Description: "Create a new AICP principal DID with Ed25519 key pair. Returns the DID document and anchor receipt.",
		InputSchema: jsonSchema(map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}),
	}, r.handleDIDCreate)

	s.RegisterTool(Tool{
		Name:        "aicp_did_resolve",
		Description: "Resolve a DID to its DID Document. Supports did:aicp, did:key, and did:web methods.",
		InputSchema: jsonSchema(map[string]any{
			"type":     "object",
			"required": []string{"did"},
			"properties": map[string]any{
				"did": map[string]any{
					"type":        "string",
					"description": "The DID to resolve (e.g., did:aicp:abc123)",
				},
			},
		}),
	}, r.handleDIDResolve)

	s.RegisterTool(Tool{
		Name:        "aicp_did_create_agent",
		Description: "Create an agent DID delegated from a principal. Requires principal DID authentication.",
		InputSchema: jsonSchema(map[string]any{
			"type":     "object",
			"required": []string{"principal_did"},
			"properties": map[string]any{
				"principal_did": map[string]any{
					"type":        "string",
					"description": "The principal DID that controls the new agent",
				},
			},
		}),
	}, r.handleDIDCreateAgent)
}

func (r *AICPToolRegistry) handleDIDCreate(ctx context.Context, _ json.RawMessage) (*ToolResult, error) {
	return r.post(ctx, "/aicp/v1/did", nil)
}

func (r *AICPToolRegistry) handleDIDResolve(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	did, err := requireStringParam(params, "did")
	if err != nil {
		return textError("%v", err), nil
	}
	return r.get(ctx, "/aicp/v1/did/"+url.PathEscape(did))
}

func (r *AICPToolRegistry) handleDIDCreateAgent(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	return r.post(ctx, "/aicp/v1/did/agent", params)
}

// ─── Discovery Tools ────────────────────────────────────────────────────────

func (r *AICPToolRegistry) registerDiscoveryTools(s *Server) {
	s.RegisterTool(Tool{
		Name:        "aicp_discover_search",
		Description: "Search the Agent Name System (ANS) for services by capability, category, or keyword.",
		InputSchema: jsonSchema(map[string]any{
			"type":     "object",
			"required": []string{"query"},
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query (capability name, keyword, or category)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of results (default: 10)",
				},
			},
		}),
	}, r.handleDiscoverSearch)

	s.RegisterTool(Tool{
		Name:        "aicp_discover_get",
		Description: "Get the ANS record for a specific service DID, including capabilities, endpoints, and trust info.",
		InputSchema: jsonSchema(map[string]any{
			"type":     "object",
			"required": []string{"did"},
			"properties": map[string]any{
				"did": map[string]any{
					"type":        "string",
					"description": "The service DID to look up",
				},
			},
		}),
	}, r.handleDiscoverGet)

	s.RegisterTool(Tool{
		Name:        "aicp_discover_register",
		Description: "Register a service in the ANS with capabilities, endpoints, and onboarding info.",
		InputSchema: jsonSchema(map[string]any{
			"type":     "object",
			"required": []string{"service"},
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "object",
					"description": "Service registration details (name, did, description, capabilities)",
				},
			},
		}),
	}, r.handleDiscoverRegister)
}

func (r *AICPToolRegistry) handleDiscoverSearch(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return textError("invalid parameters: %v", err), nil
	}
	if args.Query == "" {
		return textError("query is required"), nil
	}
	if len(args.Query) > maxInputLen {
		return textError("query exceeds maximum length"), nil
	}
	path := fmt.Sprintf("/aicp/v1/discover/search?capability=%s", url.QueryEscape(args.Query))
	if args.Limit > 0 {
		path += fmt.Sprintf("&limit=%d", args.Limit)
	}
	return r.get(ctx, path)
}

func (r *AICPToolRegistry) handleDiscoverGet(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	did, err := requireStringParam(params, "did")
	if err != nil {
		return textError("%v", err), nil
	}
	return r.get(ctx, "/aicp/v1/discover/"+url.PathEscape(did))
}

func (r *AICPToolRegistry) handleDiscoverRegister(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	return r.post(ctx, "/aicp/v1/discover/register", params)
}

// ─── Onboarding Tools ───────────────────────────────────────────────────────

func (r *AICPToolRegistry) registerOnboardingTools(s *Server) {
	s.RegisterTool(Tool{
		Name:        "aicp_onboard_initiate",
		Description: "Start a 4-phase onboarding flow with a service. Phase 1: Present identity and receive a challenge.",
		InputSchema: jsonSchema(map[string]any{
			"type":     "object",
			"required": []string{"agent_did"},
			"properties": map[string]any{
				"agent_did": map[string]any{
					"type":        "string",
					"description": "The agent DID to onboard",
				},
				"requested_capabilities": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Capabilities the agent is requesting",
				},
			},
		}),
	}, r.handleOnboardInitiate)

	s.RegisterTool(Tool{
		Name:        "aicp_onboard_negotiate",
		Description: "Phase 2: Negotiate capabilities with the service after solving the identity challenge.",
		InputSchema: jsonSchema(map[string]any{
			"type":     "object",
			"required": []string{"session_id"},
			"properties": map[string]any{
				"session_id": map[string]any{
					"type":        "string",
					"description": "The onboarding session ID from initiation",
				},
				"offered_capabilities": map[string]any{
					"type":        "array",
					"description": "Capabilities offered by the agent",
				},
			},
		}),
	}, r.handleOnboardNegotiate)

	s.RegisterTool(Tool{
		Name:        "aicp_onboard_authorize",
		Description: "Phase 3: Authorize payment terms for the service.",
		InputSchema: jsonSchema(map[string]any{
			"type":     "object",
			"required": []string{"session_id"},
			"properties": map[string]any{
				"session_id": map[string]any{
					"type":        "string",
					"description": "The onboarding session ID",
				},
			},
		}),
	}, r.handleOnboardAuthorize)

	s.RegisterTool(Tool{
		Name:        "aicp_onboard_provision",
		Description: "Phase 4: Complete onboarding and provision the agent account.",
		InputSchema: jsonSchema(map[string]any{
			"type":     "object",
			"required": []string{"session_id"},
			"properties": map[string]any{
				"session_id": map[string]any{
					"type":        "string",
					"description": "The onboarding session ID",
				},
			},
		}),
	}, r.handleOnboardProvision)

	s.RegisterTool(Tool{
		Name:        "aicp_onboard_status",
		Description: "Check the current status of an onboarding session.",
		InputSchema: jsonSchema(map[string]any{
			"type":     "object",
			"required": []string{"session_id"},
			"properties": map[string]any{
				"session_id": map[string]any{
					"type":        "string",
					"description": "The onboarding session ID to check",
				},
			},
		}),
	}, r.handleOnboardStatus)
}

func (r *AICPToolRegistry) handleOnboardInitiate(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	return r.post(ctx, "/aicp/v1/onboard", params)
}

func (r *AICPToolRegistry) handleOnboardNegotiate(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	sessionID, err := requireStringParam(params, "session_id")
	if err != nil {
		return textError("%v", err), nil
	}
	return r.post(ctx, "/aicp/v1/onboard/"+url.PathEscape(sessionID)+"/negotiate", params)
}

func (r *AICPToolRegistry) handleOnboardAuthorize(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	sessionID, err := requireStringParam(params, "session_id")
	if err != nil {
		return textError("%v", err), nil
	}
	return r.post(ctx, "/aicp/v1/onboard/"+url.PathEscape(sessionID)+"/authorize", params)
}

func (r *AICPToolRegistry) handleOnboardProvision(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	sessionID, err := requireStringParam(params, "session_id")
	if err != nil {
		return textError("%v", err), nil
	}
	return r.post(ctx, "/aicp/v1/onboard/"+url.PathEscape(sessionID)+"/provision", params)
}

func (r *AICPToolRegistry) handleOnboardStatus(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	sessionID, err := requireStringParam(params, "session_id")
	if err != nil {
		return textError("%v", err), nil
	}
	return r.get(ctx, "/aicp/v1/onboard/"+url.PathEscape(sessionID))
}

// ─── Commerce Tools ─────────────────────────────────────────────────────────

func (r *AICPToolRegistry) registerCommerceTools(s *Server) {
	s.RegisterTool(Tool{
		Name:        "aicp_payment_request_token",
		Description: "Request a payment token (JWS/EdDSA) for executing a payment to a service.",
		InputSchema: jsonSchema(map[string]any{
			"type":     "object",
			"required": []string{"service_did", "amount", "currency"},
			"properties": map[string]any{
				"service_did": map[string]any{
					"type":        "string",
					"description": "The DID of the service to pay",
				},
				"amount": map[string]any{
					"type":        "string",
					"description": "Payment amount (e.g., \"10.00\")",
				},
				"currency": map[string]any{
					"type":        "string",
					"description": "Currency code (e.g., \"USD\")",
				},
				"purpose": map[string]any{
					"type":        "string",
					"description": "Purpose of the payment",
				},
			},
		}),
	}, r.handlePaymentRequestToken)

	s.RegisterTool(Tool{
		Name:        "aicp_payment_execute",
		Description: "Execute a payment using a previously obtained payment token.",
		InputSchema: jsonSchema(map[string]any{
			"type":     "object",
			"required": []string{"payment_token"},
			"properties": map[string]any{
				"payment_token": map[string]any{
					"type":        "string",
					"description": "The JWS payment token to execute",
				},
				"adapter": map[string]any{
					"type":        "string",
					"description": "Payment adapter (stripe, paypal, usdc)",
				},
			},
		}),
	}, r.handlePaymentExecute)

	s.RegisterTool(Tool{
		Name:        "aicp_payment_status",
		Description: "Check the status of a payment by receipt ID.",
		InputSchema: jsonSchema(map[string]any{
			"type":     "object",
			"required": []string{"receipt_id"},
			"properties": map[string]any{
				"receipt_id": map[string]any{
					"type":        "string",
					"description": "The payment receipt ID",
				},
			},
		}),
	}, r.handlePaymentStatus)
}

func (r *AICPToolRegistry) handlePaymentRequestToken(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	return r.post(ctx, "/aicp/v1/payment/token", params)
}

func (r *AICPToolRegistry) handlePaymentExecute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	return r.post(ctx, "/aicp/v1/payment/execute", params)
}

func (r *AICPToolRegistry) handlePaymentStatus(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	receiptID, err := requireStringParam(params, "receipt_id")
	if err != nil {
		return textError("%v", err), nil
	}
	return r.get(ctx, "/aicp/v1/payment/status/"+url.PathEscape(receiptID))
}

// ─── Verify Tools ───────────────────────────────────────────────────────────

func (r *AICPToolRegistry) registerVerifyTools(s *Server) {
	s.RegisterTool(Tool{
		Name:        "aicp_verify",
		Description: "Verify an agent's identity, delegation chain, and trust score. Accepts a signed proof.",
		InputSchema: jsonSchema(map[string]any{
			"type":     "object",
			"required": []string{"did"},
			"properties": map[string]any{
				"did": map[string]any{
					"type":        "string",
					"description": "The DID to verify",
				},
				"proof": map[string]any{
					"type":        "object",
					"description": "Signed proof of DID control",
				},
			},
		}),
	}, r.handleVerify)

	s.RegisterTool(Tool{
		Name:        "aicp_trust_score",
		Description: "Get the trust score (0-1000) for a DID based on identity verification, transaction history, and endorsements.",
		InputSchema: jsonSchema(map[string]any{
			"type":     "object",
			"required": []string{"did"},
			"properties": map[string]any{
				"did": map[string]any{
					"type":        "string",
					"description": "The DID to check trust score for",
				},
			},
		}),
	}, r.handleTrustScore)
}

func (r *AICPToolRegistry) handleVerify(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	return r.post(ctx, "/aicp/v1/verify", params)
}

func (r *AICPToolRegistry) handleTrustScore(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	did, err := requireStringParam(params, "did")
	if err != nil {
		return textError("%v", err), nil
	}
	return r.get(ctx, "/aicp/v1/trust/"+url.PathEscape(did))
}

// ─── HTTP Helpers ───────────────────────────────────────────────────────────

func (r *AICPToolRegistry) get(ctx context.Context, path string) (*ToolResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.serverURL+path, nil)
	if err != nil {
		return textError("creating request: %v", err), nil
	}
	req.Header.Set("Accept", "application/json")
	return r.doRequest(req)
}

func (r *AICPToolRegistry) post(ctx context.Context, path string, body json.RawMessage) (*ToolResult, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.serverURL+path, bodyReader)
	if err != nil {
		return textError("creating request: %v", err), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return r.doRequest(req)
}

func (r *AICPToolRegistry) doRequest(req *http.Request) (*ToolResult, error) {
	resp, err := r.client.Do(req)
	if err != nil {
		return textError("request failed: %v", err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		return textError("reading response: %v", err), nil
	}

	// Pretty-print JSON for readability.
	var pretty bytes.Buffer
	if json.Indent(&pretty, data, "", "  ") == nil {
		data = pretty.Bytes()
	}

	isError := resp.StatusCode >= 400
	return &ToolResult{
		Content: []ToolContent{{Type: "text", Text: string(data)}},
		IsError: isError,
	}, nil
}

// ─── Parameter Helpers ──────────────────────────────────────────────────────

// requireStringParam extracts a required string field from JSON params
// with validation for emptiness and length.
func requireStringParam(params json.RawMessage, field string) (string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(params, &raw); err != nil {
		return "", fmt.Errorf("invalid parameters: %v", err)
	}
	val, ok := raw[field]
	if !ok {
		return "", fmt.Errorf("%s is required", field)
	}
	var s string
	if err := json.Unmarshal(val, &s); err != nil {
		return "", fmt.Errorf("%s must be a string", field)
	}
	if s == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if len(s) > maxInputLen {
		return "", fmt.Errorf("%s exceeds maximum length", field)
	}
	return s, nil
}

func textError(format string, args ...any) *ToolResult {
	return &ToolResult{
		Content: []ToolContent{{Type: "text", Text: fmt.Sprintf(format, args...)}},
		IsError: true,
	}
}

func jsonSchema(schema map[string]any) map[string]any {
	return schema
}
