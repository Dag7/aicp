package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

func newDiscoverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Service discovery operations",
	}

	cmd.AddCommand(newDiscoverWellKnownCmd())
	cmd.AddCommand(newDiscoverServerCmd())

	return cmd
}

func newDiscoverWellKnownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "wellknown <domain>",
		Short: "Fetch .well-known/aicp.json from a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := args[0]
			url := fmt.Sprintf("https://%s/.well-known/aicp.json", domain)

			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("fetching .well-known: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
			}

			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("reading response: %w", err)
			}

			var record map[string]any
			if err := json.Unmarshal(data, &record); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			return printOutput(record, func() {
				printANSRecord(record)
			})
		},
	}
}

func newDiscoverServerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Fetch the AICP server's .well-known info",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := http.Get(serverURL + "/.well-known/aicp.json")
			if err != nil {
				return fmt.Errorf("fetching server info: %w", err)
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
				respData = result
			}

			return printOutput(respData, func() {
				printANSRecord(respData)
			})
		},
	}
}

func printANSRecord(record map[string]any) {
	if svc, ok := record["service"].(map[string]any); ok {
		pairs := [][2]string{
			{"Service Name", fmt.Sprintf("%v", svc["name"])},
			{"Service DID", fmt.Sprintf("%v", svc["did"])},
		}
		if desc, ok := svc["description"]; ok && desc != nil {
			pairs = append(pairs, [2]string{"Description", fmt.Sprintf("%v", desc)})
		}
		if cats, ok := svc["category"].([]any); ok {
			catStrs := make([]string, len(cats))
			for i, c := range cats {
				catStrs[i] = fmt.Sprintf("%v", c)
			}
			pairs = append(pairs, [2]string{"Categories", strings.Join(catStrs, ", ")})
		}
		printKV(pairs)
	}

	if onb, ok := record["onboarding"].(map[string]any); ok {
		fmt.Println("\nOnboarding:")
		if flows, ok := onb["flows"].([]any); ok {
			flowStrs := make([]string, len(flows))
			for i, f := range flows {
				flowStrs[i] = fmt.Sprintf("%v", f)
			}
			fmt.Printf("  Flows: %s\n", strings.Join(flowStrs, ", "))
		}
		if ts, ok := onb["minimum_trust_score"].(float64); ok {
			fmt.Printf("  Min Trust Score: %.0f\n", ts)
		}
	}

	if auth, ok := record["authentication"].(map[string]any); ok {
		if methods, ok := auth["did_methods"].([]any); ok {
			methodStrs := make([]string, len(methods))
			for i, m := range methods {
				methodStrs[i] = fmt.Sprintf("%v", m)
			}
			fmt.Printf("\nDID Methods: %s\n", strings.Join(methodStrs, ", "))
		}
	}
}
