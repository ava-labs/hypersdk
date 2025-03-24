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
}

type TxWorkload struct {
	Generator TxGenerator
}

func (w *TxWorkload) GenerateBlocks(ctx context.Context, require *require.Assertions, uris []string, blocks int) {
	uri := uris[0]
	// generate [blocks] num txs
	client := jsonrpc.NewJSONRPCClient(uri)

	_, startHeight, _, err := client.Accepted(ctx)
	require.NoError(err)
	height := startHeight
	targetHeight := startHeight + uint64(blocks)

	for height < targetHeight {
		tx, confirm, err := w.Generator.GenerateTx(ctx, uri)
		require.NoError(err, "failed to generate tx at height %d", height)
		_, err = client.SubmitTx(ctx, tx.Bytes())
		require.NoError(err, "failed to submit tx at height %d", height)
		confirm(ctx, require, uri)
		_, acceptedHeight, _, err := client.Accepted(ctx)
		require.NoError(err, "failed to get accepted height at height %d", height)
		height = acceptedHeight
	}

	for _, uri := range uris[1:] {
		client := jsonrpc.NewJSONRPCClient(uri)
		acceptedHeight := uint64(0)
		err := jsonrpc.Wait(ctx, reachedAcceptedTipSleepInterval, func(ctx context.Context) (bool, error) {
			_, acceptedHeight, _, err = client.Accepted(ctx)
			if err != nil {
				return false, err
			}
			return acceptedHeight >= targetHeight, nil
		})
		require.NoError(err, "failed to reach target height %d; current height %d; uri %s", targetHeight, acceptedHeight, uri)
	}
}

// GenerateTxs generates transactions using the provided TxGenerator until the generator
// can no longer generate transactions
// issues tx through clientURI and confirms against each uri in confirmURIs
func (w *TxWorkload) GenerateTxs(ctx context.Context, require *require.Assertions, numTxs int, clientURI string, confirmUris []string) {
	submitClient := jsonrpc.NewJSONRPCClient(confirmUris[0])

	for i := 0; i < numTxs; i++ {
		tx, confirm, err := w.Generator.GenerateTx(ctx, clientURI)
		require.NoError(err)

		_, err = submitClient.SubmitTx(ctx, tx.Bytes())
		require.NoError(err)

		for _, uri := range confirmUris {
			confirm(ctx, require, uri)
		}
	}
}

func (w *TxWorkload) GenerateUntilStop(
	ctx context.Context,
	require *require.Assertions,
	uris []string,
	maxToGenerate int,
	stopChannel <-chan struct{},
) {
	submitClient := jsonrpc.NewJSONRPCClient(uris[0])
	for i := 0; i < maxToGenerate; i++ {
		select {
		case <-stopChannel:
			return
		default:
			tx, confirm, err := w.Generator.GenerateTx(ctx, uris[0])
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
