package main

import (
	"fmt"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/spf13/cobra"
)

var keyAddressCmd = &cobra.Command{
	Use:   "address",
	Short: "Print current key address",
	RunE: func(cmd *cobra.Command, args []string) error {
		keyString, err := getConfigValue(cmd, "key")
		if err != nil {
			return fmt.Errorf("failed to get key: %w", err)
		}

		key, err := privateKeyFromString(keyString)
		if err != nil {
			return fmt.Errorf("failed to decode key: %w", err)
		}

		addr := auth.NewED25519Address(key.PublicKey())
		addrString, err := addr.MarshalText()
		if err != nil {
			return fmt.Errorf("failed to marshal address: %w", err)
		}

		return printValue(keyAddressCmdResponse{
			Address: string(addrString),
		}, cmd)
	},
}

type keyAddressCmdResponse struct {
	Address string `json:"address"`
}

func (r keyAddressCmdResponse) String() string {
	return r.Address
}

func init() {
	keyCmd.AddCommand(keySetCmd, keyGenerateCmd)
}
