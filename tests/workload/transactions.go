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
	// generate [blocks] amount of txs
	client := jsonrpc.NewJSONRPCClient(uri)

	_, startHeight, _, err := client.Accepted(ctx)
	require.NoError(err)
	height := startHeight
	targetheight := startHeight + uint64(blocks)

	for count := 0; count < blocks && height < targetheight; count++ {
		tx, confirm, err := w.Generator.GenerateTx(ctx, uri)
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

// GenerateTxs generates transactions using the provided TxGenerator until the generator
// can no longer generate transactions
// ClientUri is the uri of the client that will generate the transactions
// ConfirmUris is a list of uris to confirm the transactions
func (w *TxWorkload) GenerateTxs(ctx context.Context, require *require.Assertions, amount int, clientURI string, confirmUris []string) {
	// TODO: why do we only use the first uri for submitting transactions when it differs from the confirmUris?
	submitClient := jsonrpc.NewJSONRPCClient(confirmUris[0])

	for i := 0; i < amount; i++ {
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
