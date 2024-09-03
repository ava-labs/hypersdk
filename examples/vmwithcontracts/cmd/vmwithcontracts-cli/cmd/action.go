// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"os"

	"github.com/near/borsh-go"
	"github.com/spf13/cobra"
	"github.com/status-im/keycard-go/hexutils"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/cli/prompt"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/actions"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/consts"
	"github.com/ava-labs/hypersdk/utils"
)

var actionCmd = &cobra.Command{
	Use: "action",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

func parseAddress(address string) (codec.Address, error) {
	return codec.ParseAddressBech32(consts.HRP, address)
}

var transferCmd = &cobra.Command{
	Use: "transfer",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, priv, factory, cli, bcli, ws, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Get balance info
		balance, err := handler.GetBalance(ctx, bcli, priv.Address)
		if balance == 0 || err != nil {
			return err
		}

		// Select recipient
		recipient, err := prompt.Address("recipient", parseAddress)
		if err != nil {
			return err
		}

		// Select amount
		amount, err := prompt.Amount("amount", consts.Decimals, balance, nil)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := prompt.Continue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		_, _, err = sendAndWait(ctx, []chain.Action{&actions.Transfer{
			To:    recipient,
			Value: amount,
		}}, cli, bcli, ws, factory, true)
		return err
	},
}

var publishFileCmd = &cobra.Command{
	Use: "publishFile",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, _, factory, cli, bcli, ws, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Select program bytes
		path, err := prompt.String("program file", 1, 1000)
		if err != nil {
			return err
		}
		bytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := prompt.Continue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		result, _, err := sendAndWait(ctx, []chain.Action{&actions.Publish{
			ContractBytes: bytes,
		}}, cli, bcli, ws, factory, true)

		if result != nil && result.Success {
			utils.Outf(hexutils.BytesToHex(result.Outputs[0][0]) + "\n")
		}
		return err
	},
}

var callCmd = &cobra.Command{
	Use: "call",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, priv, factory, cli, bcli, ws, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Get balance info
		balance, err := handler.GetBalance(ctx, bcli, priv.Address)
		if balance == 0 || err != nil {
			return err
		}

		// Select program
		contractAddress, err := prompt.Address("contract address", parseAddress)
		if err != nil {
			return err
		}

		// Select amount
		amount, err := prompt.Amount("amount", consts.Decimals, balance, nil)
		if err != nil {
			return err
		}

		// Select function
		function, err := prompt.String("function", 0, 100)
		if err != nil {
			return err
		}

		action := &actions.Call{
			ContractAddress: contractAddress,
			Value:           amount,
			Function:        function,
		}

		specifiedStateKeysSet, fuel, err := bcli.Simulate(ctx, *action, priv.Address)
		if err != nil {
			return err
		}

		action.SpecifiedStateKeys = make([]actions.StateKeyPermission, 0, len(specifiedStateKeysSet))
		for key, value := range specifiedStateKeysSet {
			action.SpecifiedStateKeys = append(action.SpecifiedStateKeys, actions.StateKeyPermission{Key: key, Permission: value})
		}
		action.Fuel = fuel

		// Confirm action
		cont, err := prompt.Continue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		result, _, err := sendAndWait(ctx, []chain.Action{action}, cli, bcli, ws, factory, true)

		if result != nil && result.Success {
			utils.Outf(hexutils.BytesToHex(result.Outputs[0][0]) + "\n")
			switch function {
			case "balance":
				{
					var intValue uint64
					err := borsh.Deserialize(&intValue, result.Outputs[0][0])
					if err != nil {
						return err
					}
					utils.Outf("%s\n", utils.FormatBalance(intValue, consts.Decimals))
				}
			case "get_value":
				{
					var intValue int64
					err := borsh.Deserialize(&intValue, result.Outputs[0][0])
					if err != nil {
						return err
					}
					utils.Outf("%d\n", intValue)
				}
			}
		}
		return err
	},
}

var deployCmd = &cobra.Command{
	Use: "deploy",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, _, factory, cli, bcli, ws, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		programID, err := prompt.Bytes("program id")
		if err != nil {
			return err
		}

		creationInfo, err := prompt.Bytes("creation info")
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := prompt.Continue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		result, _, err := sendAndWait(ctx, []chain.Action{&actions.Deploy{
			ProgramID:    programID,
			CreationInfo: creationInfo,
		}}, cli, bcli, ws, factory, true)

		if result != nil && result.Success {
			address, err := codec.ToAddress(result.Outputs[0][0])
			if err != nil {
				return err
			}
			addressString, err := codec.AddressBech32(consts.HRP, address)
			utils.Outf(addressString + "\n")
		}
		return err
	},
}
