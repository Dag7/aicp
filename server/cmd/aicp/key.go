package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
)

func newKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Manage Ed25519 key pairs",
	}

	cmd.AddCommand(newKeyGenerateCmd())
	cmd.AddCommand(newKeyShowCmd())

	return cmd
}

func newKeyGenerateCmd() *cobra.Command {
	var outDir string
	var name string

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a new Ed25519 key pair",
		RunE: func(cmd *cobra.Command, args []string) error {
			kp, err := crypto.GenerateEd25519KeyPair()
			if err != nil {
				return fmt.Errorf("generating key pair: %w", err)
			}

			did := crypto.PrincipalDID(kp.PublicKey)
			pubMultibase := crypto.PublicKeyMultibase(kp.PublicKey)

			if outDir != "" {
				dir := filepath.Join(outDir, name)
				if err := os.MkdirAll(dir, 0700); err != nil {
					return fmt.Errorf("creating key directory: %w", err)
				}

				// Write private key as PEM.
				privPEM := pem.EncodeToMemory(&pem.Block{
					Type:  "ED25519 PRIVATE KEY",
					Bytes: kp.PrivateKey,
				})
				privPath := filepath.Join(dir, "private.pem")
				if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
					return fmt.Errorf("writing private key: %w", err)
				}

				// Write public key as PEM.
				pubPEM := pem.EncodeToMemory(&pem.Block{
					Type:  "ED25519 PUBLIC KEY",
					Bytes: kp.PublicKey,
				})
				pubPath := filepath.Join(dir, "public.pem")
				if err := os.WriteFile(pubPath, pubPEM, 0644); err != nil {
					return fmt.Errorf("writing public key: %w", err)
				}

				fmt.Printf("Keys written to %s/\n", dir)
			}

			return printOutput(map[string]string{
				"did":         did,
				"public_key":  pubMultibase,
				"private_key": hex.EncodeToString(kp.PrivateKey[:ed25519.SeedSize]),
			}, func() {
				printKV([][2]string{
					{"DID", did},
					{"Public Key", pubMultibase},
					{"Private Key (hex)", hex.EncodeToString(kp.PrivateKey[:ed25519.SeedSize])},
				})
			})
		},
	}

	cmd.Flags().StringVar(&outDir, "dir", "", "Directory to write key files to")
	cmd.Flags().StringVar(&name, "name", "default", "Key pair name (subdirectory)")

	return cmd
}

func newKeyShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <key-file>",
		Short: "Show public key and DID from a private key file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading key file: %w", err)
			}

			block, _ := pem.Decode(data)
			if block == nil {
				return fmt.Errorf("invalid PEM file")
			}

			priv := ed25519.PrivateKey(block.Bytes)
			pub := priv.Public().(ed25519.PublicKey)
			did := crypto.PrincipalDID(pub)
			pubMultibase := crypto.PublicKeyMultibase(pub)

			return printOutput(map[string]string{
				"did":        did,
				"public_key": pubMultibase,
			}, func() {
				printKV([][2]string{
					{"DID", did},
					{"Public Key", pubMultibase},
				})
			})
		},
	}
}
