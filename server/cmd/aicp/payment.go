package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

func newPaymentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "payment",
		Aliases: []string{"pay"},
		Short:   "Payment operations",
	}

	cmd.AddCommand(newPaymentTokenCmd())
	cmd.AddCommand(newPaymentExecuteCmd())
	cmd.AddCommand(newPaymentStatusCmd())

	return cmd
}

func newPaymentTokenCmd() *cobra.Command {
	var agentDID string
	var serviceDID string
	var amount string
	var currency string
	var purpose string
	var category string

	cmd := &cobra.Command{
		Use:   "token",
		Short: "Request a payment token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentDID == "" || serviceDID == "" {
				return fmt.Errorf("--agent-did and --service-did are required")
			}
			if amount == "" || currency == "" {
				return fmt.Errorf("--amount and --currency are required")
			}

			reqBody := map[string]any{
				"agent_did":        agentDID,
				"service_did":      serviceDID,
				"amount":           map[string]string{"value": amount, "currency": currency},
				"purpose":          purpose,
				"service_category": category,
			}

			bodyBytes, _ := json.Marshal(reqBody)
			resp, err := http.Post(
				serverURL+"/aicp/v1/payment/token",
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
				printKV([][2]string{
					{"Token ID", fmt.Sprintf("%v", respData["token_id"])},
					{"Expires At", fmt.Sprintf("%v", respData["expires_at"])},
					{"Token", fmt.Sprintf("%v", respData["token"])},
				})
			})
		},
	}

	cmd.Flags().StringVar(&agentDID, "agent-did", "", "Agent DID")
	cmd.Flags().StringVar(&serviceDID, "service-did", "", "Service DID")
	cmd.Flags().StringVar(&amount, "amount", "", "Payment amount")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency")
	cmd.Flags().StringVar(&purpose, "purpose", "usage-payment", "Payment purpose")
	cmd.Flags().StringVar(&category, "category", "general", "Service category")

	return cmd
}

func newPaymentExecuteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "execute <token>",
		Short: "Execute a payment with a token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reqBody := map[string]string{"token": args[0]}
			bodyBytes, _ := json.Marshal(reqBody)

			resp, err := http.Post(
				serverURL+"/aicp/v1/payment/execute",
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
				printKV([][2]string{
					{"Payment ID", fmt.Sprintf("%v", respData["payment_id"])},
					{"Status", fmt.Sprintf("%v", respData["status"])},
					{"Receipt ID", fmt.Sprintf("%v", respData["receipt_id"])},
				})
			})
		},
	}
}

func newPaymentStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <payment-id>",
		Short: "Check payment status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := http.Get(serverURL + "/aicp/v1/payment/status/" + args[0])
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
				printKV([][2]string{
					{"Payment ID", fmt.Sprintf("%v", respData["payment_id"])},
					{"Status", fmt.Sprintf("%v", respData["status"])},
					{"Payer", fmt.Sprintf("%v", respData["payer_did"])},
					{"Payee", fmt.Sprintf("%v", respData["payee_did"])},
				})
			})
		},
	}
}
