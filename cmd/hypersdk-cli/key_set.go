// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
)

var keyGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a new key",
	RunE: func(cmd *cobra.Command, _ []string) error {
		newKey, err := ed25519.GeneratePrivateKey()
		if err != nil {
			return fmt.Errorf("failed to generate key: %w", err)
		}

		return checkAndSavePrivateKey(cmd, hex.EncodeToString(newKey[:]))
	},
}

var keySetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set the private ED25519 key",
	RunE: func(cmd *cobra.Command, _ []string) error {
		keyString, err := cmd.Flags().GetString("key")
		if err != nil {
			return fmt.Errorf("failed to get key flag: %w", err)
		}
		if keyString == "" {
			return errors.New("--key is required")
		}

		return checkAndSavePrivateKey(cmd, keyString)
	},
}

func checkAndSavePrivateKey(cmd *cobra.Command, keyStr string) error {
	key, err := privateKeyFromString(keyStr)
	if err != nil {
		return fmt.Errorf("failed to decode key: %w", err)
	}

	// Use Viper to save the key
	if err := setConfigValue("key", hex.EncodeToString(key[:])); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	addr := auth.NewED25519Address(key.PublicKey())

	addrString, err := addr.MarshalText()
	if err != nil {
		return fmt.Errorf("failed to marshal address: %w", err)
	}

	return printValue(cmd, keySetCmdResponse{
		Address: string(addrString),
	})
}

func privateKeyFromString(keyStr string) (ed25519.PrivateKey, error) {
	keyBytes, err := decodeFileOrHex(keyStr)
	if err != nil {
		return ed25519.EmptyPrivateKey, fmt.Errorf("failed to decode key: %w", err)
	}
	if len(keyBytes) != ed25519.PrivateKeyLen {
		return ed25519.EmptyPrivateKey, fmt.Errorf("invalid private key length: %d", len(keyBytes))
	}
	return ed25519.PrivateKey(keyBytes), nil
}

type keySetCmdResponse struct {
	Address string `json:"address"`
}

func (r keySetCmdResponse) String() string {
	return "✅ Key added successfully!\nAddress: " + r.Address
}

func init() {
	keyCmd.AddCommand(keySetCmd, keyGenerateCmd)
	keySetCmd.Flags().String("key", "", "Private key in hex format or path to file containing the key")

	err := keySetCmd.MarkFlagRequired("key")
	if err != nil {
		log.Fatalf("failed to mark key flag as required: %s", err)
	}
}
