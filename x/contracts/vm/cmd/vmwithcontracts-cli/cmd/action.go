// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/near/borsh-go"
	"github.com/spf13/cobra"
	"github.com/status-im/keycard-go/hexutils"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/cli/prompt"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/contracts/vm/actions"
	"github.com/ava-labs/hypersdk/x/contracts/vm/vm"
)

var errUnexpectedSimulateActionsOutput = errors.New("returned output from SimulateActions was not actions.Result")

var actionCmd = &cobra.Command{
	Use: "action",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
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
		recipient, err := prompt.Address("recipient")
		if err != nil {
			return err
		}

		// Select amount
		amount, err := prompt.Amount("amount", balance, nil)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := prompt.Continue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		_, err = sendAndWait(ctx, []chain.Action{&actions.Transfer{
			To:    recipient,
			Value: amount,
		}}, cli, bcli, ws, factory)
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

		// Select contract bytes
		path, err := prompt.String("contract file", 1, 1000)
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
		result, err := sendAndWait(ctx, []chain.Action{&actions.Publish{
			ContractBytes: bytes,
		}}, cli, bcli, ws, factory)

		if result != nil && result.Success {
			utils.Outf(hexutils.BytesToHex(result.Outputs[0]) + "\n")
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

		// Select contract
		contractAddress, err := prompt.Address("contract address")
		if err != nil {
			return err
		}

		// Select amount
		amount, err := prompt.Amount("amount", balance, nil)
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
			Fuel:            uint64(1000000000),
		}

		actionSimulationResults, err := cli.SimulateActions(ctx, chain.Actions{action}, priv.Address)
		if err != nil {
			return err
		}
		if len(actionSimulationResults) != 1 {
			return fmt.Errorf("unexpected number of returned actions. One action expected, %d returned", len(actionSimulationResults))
		}
		actionSimulationResult := actionSimulationResults[0]

		rtx := codec.NewReader(actionSimulationResult.Output, len(actionSimulationResult.Output))

		simulationResultOutput, err := (*vm.OutputParser).Unmarshal(rtx)
		if err != nil {
			return err
		}
		simulationResult, ok := simulationResultOutput.(*actions.Result)
		if !ok {
			return errUnexpectedSimulateActionsOutput
		}

		action.SpecifiedStateKeys = make([]actions.StateKeyPermission, 0, len(actionSimulationResult.StateKeys))
		for key, value := range actionSimulationResult.StateKeys {
			action.SpecifiedStateKeys = append(action.SpecifiedStateKeys, actions.StateKeyPermission{Key: key, Permission: value})
		}
		action.Fuel = simulationResult.ConsumedFuel

		// Confirm action
		cont, err := prompt.Continue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		result, err := sendAndWait(ctx, []chain.Action{action}, cli, bcli, ws, factory)

		if result != nil && result.Success {
			utils.Outf(hexutils.BytesToHex(result.Outputs[0]) + "\n")
			switch function {
			case "balance":
				{
					var intValue uint64
					err := borsh.Deserialize(&intValue, result.Outputs[0])
					if err != nil {
						return err
					}
					utils.Outf("%s\n", utils.FormatBalance(intValue))
				}
			case "get_value":
				{
					var intValue int64
					err := borsh.Deserialize(&intValue, result.Outputs[0])
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

		contractID, err := prompt.Bytes("contract id")
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
		result, err := sendAndWait(ctx, []chain.Action{&actions.Deploy{
			ContractID:   contractID,
			CreationInfo: creationInfo,
		}}, cli, bcli, ws, factory)

		if result != nil && result.Success {
			address, err := codec.ToAddress(result.Outputs[0])
			if err != nil {
				return err
			}
			utils.Outf(address.String() + "\n")
		}
		return err
	},
}
