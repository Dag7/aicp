package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

func newDIDCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "did",
		Short: "DID identity operations",
	}

	cmd.AddCommand(newDIDResolveCmd())
	cmd.AddCommand(newDIDVerifyCmd())

	return cmd
}

func newDIDResolveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resolve <did>",
		Short: "Resolve a DID and display its document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			did := args[0]

			body := fmt.Sprintf(`{"did":"%s"}`, did)
			resp, err := http.Post(
				serverURL+"/aicp/v1/verify",
				"application/json",
				strings.NewReader(body),
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
				valid, _ := respData["valid"].(bool)
				didStr, _ := respData["did"].(string)
				trustScore, _ := respData["trust_score"].(float64)

				validStr := "false"
				if valid {
					validStr = "true"
				}

				printKV([][2]string{
					{"DID", didStr},
					{"Valid", validStr},
					{"Trust Score", fmt.Sprintf("%.0f", trustScore)},
				})

				if errs, ok := respData["errors"].([]any); ok && len(errs) > 0 {
					fmt.Println("\nErrors:")
					for _, e := range errs {
						fmt.Printf("  - %v\n", e)
					}
				}
			})
		},
	}
}

func newDIDVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify <did>",
		Short: "Verify a DID (alias for resolve with validation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Same as resolve — the verify endpoint does both.
			return newDIDResolveCmd().RunE(cmd, args)
		},
	}
}
