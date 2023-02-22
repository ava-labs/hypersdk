// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"

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
	Use:   "transfer [options] <to> <value>",
	Short: "Transfers value to another address",
	RunE:  transferFunc,
}

func transferFunc(_ *cobra.Command, args []string) error {
	priv, err := crypto.LoadKey(privateKeyFile)
	if err != nil {
		return err
	}
	factory := auth.NewDirectFactory(priv)

	to, value, err := getTransferOp(args)
	if err != nil {
		return err
	}

	ctx := context.Background()
	cli := client.New(uri)
	submit, tx, _, err := cli.GenerateTransaction(ctx, &actions.Transfer{
		To:    to,
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

func getTransferOp(args []string) (to crypto.PublicKey, value uint64, err error) {
	if len(args) != 2 {
		return crypto.EmptyPublicKey, 0, fmt.Errorf(
			"expected exactly 2 arguments, got %d",
			len(args),
		)
	}

	addr, err := utils.ParseAddress(args[0])
	if err != nil {
		return crypto.EmptyPublicKey, 0, fmt.Errorf("%w: failed to parse address %s", err, args[0])
	}
	value, err = hutils.ParseBalance(args[1])
	if err != nil {
		return crypto.EmptyPublicKey, 0, fmt.Errorf("%w: failed to parse %s", err, args[1])
	}
	return addr, value, nil
}
