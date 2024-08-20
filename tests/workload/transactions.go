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

// TxWorkloadFactory prescribes an exact interface for generating transactions to test on a given environment
// and a sized sequence of transactions to test on a given environment and reach a particular state
type TxWorkloadFactory interface {
	// NewWorkloads returns a set of TxWorkloadIterators from the VM. VM developers can use this function
	// to define each sequence of transactions that should be tested.
	// TODO: switch from workload generator to procedural test style for VM-defined workloads
	NewWorkloads(uri string) ([]TxWorkloadIterator, error)
	// Generates a new TxWorkloadIterator that generates a sequence of transactions of the given size.
	NewSizedTxWorkload(uri string, size int) (TxWorkloadIterator, error)
}

type TxAssertion func(ctx context.Context, uri string) error

// TxWorkloadIterator provides an interface for generating a sequence of transactions and corresponding assertions.
// The caller must proceed in the following sequence:
// 1. Next
// 2. GenerateTxWithAssertion
// 3. (Optional) execute the returned assertion on an arbitrary number of URIs
//
// This pattern allows the workload to define how many transactions must be generated. For example,
// a CFMM application may define a set of workloads that define each sequence of actions that should be tested
// against different network configurations such as:
// 1. Create Liquidity Pool
// 2. Add Liquidity
// 3. Swap
//
// To handle tx expiry correctly, the workload must generate txs on demand (right before issuance) rather than
// returning a slice of txs, which may expire before they are issued.
type TxWorkloadIterator interface {
	// Next returns true iff there are more transactions to generate.
	Next() bool
	// GenerateTxWithAssertion generates a new transaction and an assertion function that confirms
	// 1. The tx was accepted on the provided URI
	// 2. The state was updated as expected according to the provided URI
	GenerateTxWithAssertion(context.Context) (*chain.Transaction, TxAssertion, error)
}

func ExecuteWorkload(ctx context.Context, require *require.Assertions, uris []string, generator TxWorkloadIterator) {
	submitClient := jsonrpc.NewJSONRPCClient(uris[0])

	for generator.Next() {
		tx, confirm, err := generator.GenerateTxWithAssertion(ctx)
		require.NoError(err)

		_, err = submitClient.SubmitTx(ctx, tx.Bytes())
		require.NoError(err)

		for _, uri := range uris {
			err := confirm(ctx, uri)
			require.NoError(err)
		}
	}
}

func GenerateNBlocks(ctx context.Context, require *require.Assertions, uris []string, factory TxWorkloadFactory, n uint64) {
	uri := uris[0]
	generator, err := factory.NewSizedTxWorkload(uri, int(n))
	require.NoError(err)
	client := jsonrpc.NewJSONRPCClient(uri)

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

	for _, uri := range uris {
		client := jsonrpc.NewJSONRPCClient(uri)
		err := jsonrpc.Wait(ctx, func(ctx context.Context) (bool, error) {
			_, acceptedHeight, _, err := client.Accepted(ctx)
			if err != nil {
				return false, err
			}
			return acceptedHeight >= targetheight, nil
		})
		require.NoError(err)
	}
}

func GenerateUntilCancel(
	ctx context.Context,
	uris []string,
	generator TxWorkloadIterator,
) {
	submitClient := jsonrpc.NewJSONRPCClient(uris[0])

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

		for _, uri := range uris {
			_ = confirm(backgroundCtx, uri)
		}
	}
}
