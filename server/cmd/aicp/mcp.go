package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/aicp-protocol/aicp/server/internal/mcp"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start an MCP server exposing AICP as tools",
		Long: `Start a Model Context Protocol (MCP) server that exposes AICP
protocol operations as MCP tools. Uses JSON-RPC 2.0 over stdio.

This allows AI agents and LLMs (like Claude) to interact with AICP
services for identity, discovery, onboarding, and commerce.

Tools exposed:
  - aicp_did_create, aicp_did_resolve, aicp_did_create_agent
  - aicp_discover_search, aicp_discover_get, aicp_discover_register
  - aicp_onboard_initiate, aicp_onboard_negotiate, aicp_onboard_authorize, aicp_onboard_provision, aicp_onboard_status
  - aicp_payment_request_token, aicp_payment_execute, aicp_payment_status
  - aicp_verify, aicp_trust_score`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer cancel()

			server := mcp.NewServer(logger)

			// Register AICP tools that forward to the REST API.
			registry := mcp.NewAICPToolRegistry(serverURL)
			registry.RegisterAll(server)

			logger.Info("starting AICP MCP server", "aicp_server", serverURL)
			return server.Run(ctx)
		},
	}

	return cmd
}
