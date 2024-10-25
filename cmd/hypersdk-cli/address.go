// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/auth"
)

var addressCmd = &cobra.Command{
	Use:   "address",
	Short: "Print current key address",
	RunE: func(cmd *cobra.Command, _ []string) error {
		keyString, err := getConfigValue(cmd, "key", true)
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

		return printValue(cmd, keyAddressCmdResponse{
			Address: string(addrString),
		})
	},
}

type keyAddressCmdResponse struct {
	Address string `json:"address"`
}

func (r keyAddressCmdResponse) String() string {
	return r.Address
}

func init() {
	rootCmd.AddCommand(addressCmd)
}
