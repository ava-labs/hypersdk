// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
)

const reachedAcceptedTipSleepInterval = 10 * time.Millisecond

type TxAssertion func(ctx context.Context, require *require.Assertions, uri string)

type TxGenerator interface {
	// GenerateTx generates a new transaction and an assertion function that confirms
	// the transaction was accepted by the network
	GenerateTx(context.Context, string) (*chain.Transaction, TxAssertion, error)

	// CanGenerate returns true if there are more transactions to generate
	CanGenerate() bool

	// NewTxGenerator returns a new TxGenerator with the same configuration as the current TxGenerator
	NewTxGenerator(maxToGenerate int) TxGenerator
}

type TxWorkload struct {
	Generator TxGenerator
} 

func (w *TxWorkload) GenerateBlocks(ctx context.Context, require *require.Assertions, uris []string, blocks int) {
	uri := uris[0]
	// generate [blocks] amount of txs
	generator := w.Generator.NewTxGenerator(blocks)
	client := jsonrpc.NewJSONRPCClient(uri)

	_, startHeight, _, err := client.Accepted(ctx)
	require.NoError(err)
	height := startHeight
	targetheight := startHeight + uint64(blocks)

	for generator.CanGenerate() && height < targetheight {
		tx, confirm, err := generator.GenerateTx(ctx, uri)
		require.NoError(err)

		_, err = client.SubmitTx(ctx, tx.Bytes())
		require.NoError(err)

		confirm(ctx, require, uri)

		_, acceptedHeight, _, err := client.Accepted(ctx)
		require.NoError(err)
		height = acceptedHeight
	}

	for _, uri := range uris {
		client := jsonrpc.NewJSONRPCClient(uri)
		err := jsonrpc.Wait(ctx, reachedAcceptedTipSleepInterval, func(ctx context.Context) (bool, error) {
			_, acceptedHeight, _, err := client.Accepted(ctx)
			if err != nil {
				return false, err
			}
			return acceptedHeight >= targetheight, nil
		})
		require.NoError(err)
	}

}

func GenerateTxs(ctx context.Context, require *require.Assertions, uris []string, generator TxGenerator) {
	submitClient := jsonrpc.NewJSONRPCClient(uris[0])

	for generator.CanGenerate() {
		tx, confirm, err := generator.GenerateTx(ctx, uris[0])
		require.NoError(err)

		_, err = submitClient.SubmitTx(ctx, tx.Bytes())
		require.NoError(err)

		for _, uri := range uris {
			confirm(ctx, require, uri)
		}
	}
}

func GenerateUntilStop(
	ctx context.Context,
	require *require.Assertions,
	uris []string,
	generator TxGenerator,
	stopChannel <-chan struct{},
) {
	submitClient := jsonrpc.NewJSONRPCClient(uris[0])
	for {
		select {
		case <-stopChannel:
			return
		default:
			if !generator.CanGenerate() {
				return
			}
			tx, confirm, err := generator.GenerateTx(ctx, uris[0])
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}

			_, err = submitClient.SubmitTx(ctx, tx.Bytes())
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}

			for _, uri := range uris {
				confirm(ctx, require, uri)
			}
		}
	}
}
