// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
)

var actionCmd = &cobra.Command{
	Use: "action",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var trasnferCmd = &cobra.Command{
	Use: "transfer",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		priv, err := GetDefaultKey()
		if err != nil {
			return err
		}
		if priv == crypto.EmptyPrivateKey {
			return nil
		}
		factory := auth.NewED25519Factory(priv)
		uri, err := GetDefaultChain()
		if err != nil {
			return err
		}
		if len(uri) == 0 {
			return nil
		}
		cli := client.New(uri)

		// Select token to send
		assetID, err := promptAsset()
		if err != nil {
			return err
		}
		addr := utils.Address(priv.PublicKey())
		balance, err := cli.Balance(ctx, addr, assetID)
		if err != nil {
			return err
		}
		if balance == 0 {
			hutils.Outf("{{red}}balance:{{/}} 0 %s\n", assetString(assetID))
			hutils.Outf("{{red}}please send funds to %s{{/}}\n", addr)
			hutils.Outf("{{red}}exiting...{{/}}\n")
			return nil
		}
		hutils.Outf("{{yellow}}balance:{{/}} %s %s\n", valueString(assetID, balance), assetString(assetID))

		// Select recipient
		recipient, err := promptAddress("recipient")
		if err != nil {
			return err
		}

		// Select amount
		amount, err := promptAmount(assetID, balance)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := promptContinue()
		if !cont || err != nil {
			return err
		}

		submit, tx, _, err := cli.GenerateTransaction(ctx, nil, &actions.Transfer{
			To:    recipient,
			Asset: assetID,
			Value: amount,
		}, factory)
		if err != nil {
			return err
		}
		if err := submit(ctx); err != nil {
			return err
		}
		success, err := cli.WaitForTransaction(ctx, tx.ID())
		if err != nil {
			return err
		}
		printStatus(tx.ID(), success)
		return nil
	},
}
