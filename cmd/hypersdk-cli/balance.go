// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/codec"
)

var balanceCmd = &cobra.Command{
	Use:   "balance [address]",
	Short: "Get the balance of an address",
	RunE: func(cmd *cobra.Command, _ []string) error {
		// 1. figure out sender address
		addressStr, err := cmd.Flags().GetString("sender")
		if err != nil {
			return fmt.Errorf("failed to get sender: %w", err)
		}

		var address codec.Address

		if addressStr != "" {
			address, err = codec.StringToAddress(addressStr)
			if err != nil {
				return fmt.Errorf("failed to convert sender to address: %w", err)
			}
		} else {
			// ok, infer user's address from the private key
			keyString, err := getConfigValue(cmd, "key", true)
			if err != nil {
				return fmt.Errorf("failed to get key from config: %w", err)
			}
			key, err := privateKeyFromString(keyString)
			if err != nil {
				return fmt.Errorf("failed to decode key: %w", err)
			}
			address = auth.NewED25519Address(key.PublicKey())
		}

		// 2. create client
		endpoint, err := getConfigValue(cmd, "endpoint", true)
		if err != nil {
			return fmt.Errorf("failed to get endpoint: %w", err)
		}
		client := jsonrpc.NewJSONRPCClient(endpoint)

		// 3. get balance
		balance, err := client.GetBalance(context.Background(), address)
		errorString := ""
		if err != nil {
			errorString = err.Error()
		}

		return printValue(cmd, balanceResponse{
			Balance:      balance,
			BalanceError: errorString,
		})
	},
}

type balanceResponse struct {
	Balance      uint64 `json:"balance"`
	BalanceError string `json:"error"`
}

func (b balanceResponse) String() string {
	var result strings.Builder
	if b.BalanceError != "" {
		result.WriteString(fmt.Sprintf("❌ Error: %s\n", b.BalanceError))
	} else {
		result.WriteString(fmt.Sprintf("✅ Balance: %d\n", b.Balance))
	}

	return result.String()
}

func init() {
	balanceCmd.Flags().String("sender", "", "Address being queried in hex")
	rootCmd.AddCommand(balanceCmd)
}
