package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

func newOnboardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "onboard",
		Short: "Onboarding operations",
	}

	cmd.AddCommand(newOnboardInitiateCmd())
	cmd.AddCommand(newOnboardStatusCmd())

	return cmd
}

func newOnboardInitiateCmd() *cobra.Command {
	var agentDID string
	var capabilities []string

	cmd := &cobra.Command{
		Use:   "initiate",
		Short: "Initiate an onboarding flow",
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentDID == "" {
				return fmt.Errorf("--agent-did is required")
			}

			reqBody := map[string]any{
				"agent_did": agentDID,
				"presentation": map[string]any{
					"@context":             []string{"https://www.w3.org/2018/credentials/v1"},
					"type":                 []string{"VerifiablePresentation"},
					"holder":               agentDID,
					"verifiableCredential": []any{},
				},
				"requested_capabilities": capabilities,
			}

			bodyBytes, _ := json.Marshal(reqBody)
			resp, err := http.Post(
				serverURL+"/aicp/v1/onboard",
				"application/json",
				bytes.NewReader(bodyBytes),
			)
			if err != nil {
				return fmt.Errorf("request failed: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("reading response: %w", err)
			}

			var result map[string]any
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			respData, ok := result["data"].(map[string]any)
			if !ok {
				return fmt.Errorf("server error: %s", string(data))
			}

			return printOutput(respData, func() {
				if sid, ok := respData["session_id"].(string); ok {
					fmt.Printf("Session ID: %s\n", sid)
				}
				if state, ok := respData["state"].(map[string]any); ok {
					fmt.Printf("Phase: %v\n", state["phase"])
					fmt.Printf("Expires: %v\n", state["expires_at"])
				}
				if _, ok := respData["challenge"]; ok {
					fmt.Println("\nChallenge issued. Use 'aicp onboard negotiate' to continue.")
				}
			})
		},
	}

	cmd.Flags().StringVar(&agentDID, "agent-did", "", "Agent DID to onboard")
	cmd.Flags().StringSliceVar(&capabilities, "capabilities", nil, "Requested capabilities")

	return cmd
}

func newOnboardStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <session-id>",
		Short: "Check onboarding session status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]

			resp, err := http.Get(serverURL + "/aicp/v1/onboard/" + sessionID)
			if err != nil {
				return fmt.Errorf("request failed: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("reading response: %w", err)
			}

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("server error (HTTP %d): %s", resp.StatusCode, string(data))
			}

			var result map[string]any
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			respData, ok := result["data"].(map[string]any)
			if !ok {
				return fmt.Errorf("unexpected response format: %s", string(data))
			}

			return printOutput(respData, func() {
				fmt.Printf("Session:    %v\n", respData["session_id"])
				fmt.Printf("Phase:      %v\n", respData["phase"])
				fmt.Printf("Agent DID:  %v\n", respData["agent_did"])
				fmt.Printf("Service:    %v\n", respData["service_did"])
				fmt.Printf("Started:    %v\n", respData["started_at"])
				fmt.Printf("Updated:    %v\n", respData["updated_at"])
				fmt.Printf("Expires:    %v\n", respData["expires_at"])
				if reason, ok := respData["failed_reason"].(string); ok && reason != "" {
					fmt.Printf("Failure:    %s\n", reason)
				}
			})
		},
	}
}
