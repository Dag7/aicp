package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Set at build time via -ldflags.
	version = "dev"
	commit  = "none"
)

// Global flags.
var (
	serverURL string
	output    string
	keyFile   string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "aicp",
		Short: "AICP CLI — Agentic Identity and Commerce Protocol",
		Long: `AICP CLI provides commands for managing agent identities,
discovering services, onboarding, and making payments on
the AICP protocol.`,
		SilenceUsage: true,
	}

	// Global flags.
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "http://localhost:8080", "AICP server URL")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "table", "Output format: table, json, yaml")
	rootCmd.PersistentFlags().StringVar(&keyFile, "key", "", "Path to Ed25519 private key file")

	// Register sub-commands.
	rootCmd.AddCommand(
		newVersionCmd(),
		newDIDCmd(),
		newCredentialCmd(),
		newDiscoverCmd(),
		newOnboardCmd(),
		newPaymentCmd(),
		newKeyCmd(),
		newHealthCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("aicp %s (commit: %s)\n", version, commit)
		},
	}
}
