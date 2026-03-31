package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCredentialCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "credential",
		Aliases: []string{"cred"},
		Short:   "Verifiable credential operations",
	}

	cmd.AddCommand(newCredentialVerifyCmd())

	return cmd
}

func newCredentialVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify <did>",
		Short: "Verify credentials for a DID (via the verify endpoint)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Uses the same verify endpoint.
			fmt.Printf("Verifying credentials for %s against %s...\n", args[0], serverURL)
			return newDIDResolveCmd().RunE(cmd, args)
		},
	}
}
