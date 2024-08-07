// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/status-im/keycard-go/hexutils"
	"os"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/programsvm/actions"
	"github.com/ava-labs/hypersdk/examples/programsvm/consts"
)

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
		recipient, err := handler.Root().PromptAddress("recipient")
		if err != nil {
			return err
		}

		// Select amount
		amount, err := handler.Root().PromptAmount("amount", consts.Decimals, balance, nil)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
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

var publishProgramFileCmd = &cobra.Command{
	Use: "publishProgramFile",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, _, factory, cli, bcli, ws, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Select program bytes
		path, err := handler.Root().PromptString("program file", 1, 1000)
		if err != nil {
			return err
		}
		bytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		result, _, err := sendAndWait(ctx, []chain.Action{&actions.PublishProgram{
			ProgramBytes: bytes,
		}}, cli, bcli, ws, factory, true)

		if result != nil && result.Success {
			utils.Outf(hexutils.BytesToHex(result.Outputs[0][0]) + "\n")
		}
		return err
	},
}

var callProgramCmd = &cobra.Command{
	Use: "callProgram",
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
		programAccount, err := handler.Root().PromptAddress("program")
		if err != nil {
			return err
		}

		// Select amount
		amount, err := handler.Root().PromptAmount("amount", consts.Decimals, balance, nil)
		if err != nil {
			return err
		}

		// Select function
		function, err := handler.Root().PromptString("function", 0, 100)
		if err != nil {
			return err
		}

		action := &actions.CallProgram{
			Program:  programAccount,
			Value:    amount,
			Function: function,
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
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		result, _, err := sendAndWait(ctx, []chain.Action{action}, cli, bcli, ws, factory, true)

		if result != nil && result.Success {
			utils.Outf(hexutils.BytesToHex(result.Outputs[0][0]) + "\n")
		}
		return err
	},
}

var deployProgramCmd = &cobra.Command{
	Use: "deployProgram",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, _, factory, cli, bcli, ws, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		programID, err := handler.Root().PromptBytes("program id")
		if err != nil {
			return err
		}

		creationInfo, err := handler.Root().PromptBytes("creation info")
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		result, _, err := sendAndWait(ctx, []chain.Action{&actions.DeployProgram{
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
