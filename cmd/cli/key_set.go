package main

import (
	"encoding/hex"
	"fmt"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/spf13/cobra"
)

var keyGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a new key",
	RunE: func(cmd *cobra.Command, args []string) error {
		newKey, err := ed25519.GeneratePrivateKey()
		if err != nil {
			return fmt.Errorf("failed to generate key: %w", err)
		}

		return checkAndSavePrivateKey(hex.EncodeToString(newKey[:]), cmd)
	},
}

var keySetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set the private ED25519 key",
	RunE: func(cmd *cobra.Command, args []string) error {
		//read directly from the flag instead of calling getConfigValue
		keyString, err := cmd.Flags().GetString("key")
		if err != nil {
			return fmt.Errorf("failed to get key: %w", err)
		}
		if keyString == "" {
			return fmt.Errorf("--key is required")
		}

		return checkAndSavePrivateKey(keyString, cmd)
	},
}

func checkAndSavePrivateKey(keyString string, cmd *cobra.Command) error {
	key, err := privateKeyFromString(keyString)
	if err != nil {
		return fmt.Errorf("failed to decode key: %w", err)
	}

	err = updateConfig("key", hex.EncodeToString(key[:]))
	if err != nil {
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
	return fmt.Sprintf("✅ Key added successfully!\nAddress: %s", r.Address)
}

func init() {
	keyCmd.AddCommand(keySetCmd, keyGenerateCmd)
}
