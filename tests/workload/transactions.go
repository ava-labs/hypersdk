// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
)

// TxWorkloadFactory prescribes an exact interface for generating transactions to test on a given environment
// and a sized sequence of transactions to test on a given environment and reach a particular state
type TxWorkloadFactory interface {
	// Generate a new TxWorkloadIterator that generates a single sequence of transactions
	// and corresponding assertions.
	NewBasicTxWorkload(uri string) (TxWorkloadIterator, error)
	// Generates a new TxWorkloadIterator that generates a sequence of transactions of the given size.
	NewSizedTxWorkload(uri string, size int) (TxWorkloadIterator, error)
}

// TxWorkloadIterator provides an interface for generating a sequence of transactions and corresponding assertions.
// The caller must proceed in the following sequence:
// 1. Next
// 2. GenerateTxWithAssertion
// 3. (Optional) execute the returned assertion on an arbitrary number of URIs
type TxWorkloadIterator interface {
	// Next returns true iff there are more transactions to generate.
	Next() bool
	// GenerateTxWithAssertion generates a new transaction and an assertion function that confirms
	// 1. The tx was accepted on the provided URI
	// 2. The state was updated as expected according to the provided URI
	GenerateTxWithAssertion(context.Context) (*chain.Transaction, func(ctx context.Context, uri string) error, error)
}

func ExecuteWorkload(ctx context.Context, require *require.Assertions, network Network, generator TxWorkloadIterator) {
	submitClient := rpc.NewJSONRPCClient(network.URIs()[0])

	for generator.Next() {
		tx, confirm, err := generator.GenerateTxWithAssertion(ctx)
		require.NoError(err)

		_, err = submitClient.SubmitTx(ctx, tx.Bytes())
		require.NoError(err)

		utils.ForEach(func(uri string) {
			err := confirm(ctx, uri)
			require.NoError(err)
		}, network.URIs())
	}
}

func GenerateNBlocks(ctx context.Context, require *require.Assertions, network Network, factory TxWorkloadFactory, n uint64) {
	generator, err := factory.NewSizedTxWorkload(network.URIs()[0], int(n))
	require.NoError(err)
	uri := network.URIs()[0]
	client := rpc.NewJSONRPCClient(uri)

	_, startHeight, _, err := client.Accepted(ctx)
	require.NoError(err)
	height := startHeight
	targetheight := startHeight + n

	for generator.Next() && height < targetheight {
		tx, confirm, err := generator.GenerateTxWithAssertion(ctx)
		require.NoError(err)

		_, err = client.SubmitTx(ctx, tx.Bytes())
		require.NoError(err)

		require.NoError(confirm(ctx, uri))

		_, acceptedHeight, _, err := client.Accepted(ctx)
		require.NoError(err)
		height = acceptedHeight
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	utils.ForEach(func(uri string) {
		client := rpc.NewJSONRPCClient(uri)
		err := rpc.Wait(ctx, func(ctx context.Context) (bool, error) {
			_, acceptedHeight, _, err := client.Accepted(ctx)
			if err != nil {
				return false, err
			}
			return acceptedHeight >= targetheight, nil
		})
		require.NoError(err)
	}, network.URIs())
}

func GenerateUntilCancel(
	ctx context.Context,
	network Network,
	generator TxWorkloadIterator,
) {
	submitClient := rpc.NewJSONRPCClient(network.URIs()[0])

	// Use backgroundCtx within the loop to avoid erroring due to an expected
	// context cancellation.
	backgroundCtx := context.Background()
	for generator.Next() && ctx.Err() == nil {
		tx, confirm, err := generator.GenerateTxWithAssertion(backgroundCtx)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		_, err = submitClient.SubmitTx(backgroundCtx, tx.Bytes())
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		utils.ForEach(func(uri string) {
			_ = confirm(backgroundCtx, uri)
		}, network.URIs())
	}
}
