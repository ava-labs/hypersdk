// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/crypto"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
)

var transferCmd = &cobra.Command{
	Use:   "transfer [options] <to> <asset> <value>",
	Short: "Transfers value to another address",
	RunE:  transferFunc,
}

func transferFunc(_ *cobra.Command, args []string) error {
	priv, err := crypto.LoadKey(privateKeyFile)
	if err != nil {
		return err
	}
	factory := auth.NewED25519Factory(priv)

	to, asset, value, err := getTransferOp(args)
	if err != nil {
		return err
	}

	ctx := context.Background()
	cli := client.New(uri)
	submit, tx, _, err := cli.GenerateTransaction(ctx, &actions.Transfer{
		To:    to,
		Asset: asset,
		Value: value,
	}, factory)
	if err != nil {
		return err
	}
	if err := submit(ctx); err != nil {
		return err
	}
	if err := cli.WaitForTransaction(ctx, tx.ID()); err != nil {
		return err
	}
	color.Green("transferred %s to %s", hutils.FormatBalance(value), utils.Address(to))
	return nil
}

func getTransferOp(args []string) (crypto.PublicKey, ids.ID, uint64, error) {
	if len(args) != 3 {
		return crypto.EmptyPublicKey, ids.Empty, 0, fmt.Errorf(
			"expected exactly 2 arguments, got %d",
			len(args),
		)
	}

	addr, err := utils.ParseAddress(args[0])
	if err != nil {
		return crypto.EmptyPublicKey, ids.Empty, 0, fmt.Errorf(
			"%w: failed to parse address %s",
			err,
			args[0],
		)
	}
	asset, err := ids.FromString(args[1])
	if err != nil {
		return crypto.EmptyPublicKey, ids.Empty, 0, fmt.Errorf(
			"%w: failed to parse asset %s",
			err,
			args[1],
		)
	}
	value, err := hutils.ParseBalance(args[2])
	if err != nil {
		return crypto.EmptyPublicKey, ids.Empty, 0, fmt.Errorf(
			"%w: failed to parse %s",
			err,
			args[2],
		)
	}
	return addr, asset, value, nil
}
