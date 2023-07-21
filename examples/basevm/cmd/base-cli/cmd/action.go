// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"time"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/basevm/actions"
	trpc "github.com/ava-labs/hypersdk/examples/basevm/rpc"
	"github.com/ava-labs/hypersdk/rpc"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
)

const (
	dummyBlockAgeThreshold = 25
	dummyHeightThreshold   = 3
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
		_, priv, factory, cli, tcli, err := defaultActor()
		if err != nil {
			return err
		}

		// Get balance info
		balance, err := getBalance(ctx, tcli, priv.PublicKey())
		if balance == 0 || err != nil {
			return err
		}

		// Select recipient
		recipient, err := promptAddress("recipient")
		if err != nil {
			return err
		}

		// Select amount
		amount, err := promptAmount("amount", balance, nil)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := promptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		parser, err := tcli.Parser(ctx)
		if err != nil {
			return err
		}
		submit, tx, _, err := cli.GenerateTransaction(ctx, parser, nil, &actions.Transfer{
			To:    recipient,
			Value: amount,
		}, factory)
		if err != nil {
			return err
		}
		if err := submit(ctx); err != nil {
			return err
		}
		success, err := tcli.WaitForTransaction(ctx, tx.ID())
		if err != nil {
			return err
		}
		printStatus(tx.ID(), success)
		return nil
	},
}

func submitDummy(
	ctx context.Context,
	cli *rpc.JSONRPCClient,
	tcli *trpc.JSONRPCClient,
	dest crypto.PublicKey,
	factory chain.AuthFactory,
) error {
	var (
		logEmitted bool
		txsSent    uint64
	)
	for ctx.Err() == nil {
		_, h, t, err := cli.Accepted(ctx)
		if err != nil {
			return err
		}
		underHeight := h < dummyHeightThreshold
		if underHeight || time.Now().Unix()-t > dummyBlockAgeThreshold {
			if underHeight && !logEmitted {
				hutils.Outf(
					"{{yellow}}waiting for snowman++ activation (needed for AWM)...{{/}}\n",
				)
				logEmitted = true
			}
			parser, err := tcli.Parser(ctx)
			if err != nil {
				return err
			}
			submit, tx, _, err := cli.GenerateTransaction(ctx, parser, nil, &actions.Transfer{
				To:    dest,
				Value: txsSent + 1, // prevent duplicate txs
			}, factory)
			if err != nil {
				return err
			}
			if err := submit(ctx); err != nil {
				return err
			}
			if _, err := tcli.WaitForTransaction(ctx, tx.ID()); err != nil {
				return err
			}
			txsSent++
			time.Sleep(750 * time.Millisecond)
			continue
		}
		if logEmitted {
			hutils.Outf("{{yellow}}snowman++ activated{{/}}\n")
		}
		return nil
	}
	return ctx.Err()
}
