package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

func newHealthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check server health",
	}

	cmd.AddCommand(newHealthCheckCmd())
	cmd.AddCommand(newHealthReadyCmd())

	return cmd
}

func newHealthCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check liveness",
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkEndpoint(serverURL + "/health")
		},
	}
}

func newHealthReadyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ready",
		Short: "Check readiness (includes database connectivity)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkEndpoint(serverURL + "/ready")
		},
	}
}

func checkEndpoint(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("UNHEALTHY: %v\n", err)
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if output == "json" {
		var result any
		json.Unmarshal(data, &result)
		return printJSON(result)
	}

	if resp.StatusCode == http.StatusOK {
		fmt.Println("HEALTHY")
	} else {
		fmt.Printf("UNHEALTHY (HTTP %d)\n", resp.StatusCode)
	}
	return nil
}
