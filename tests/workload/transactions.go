// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
)

const reachedAcceptedTipSleepInterval = 10 * time.Millisecond

// TxWorkloadFactory prescribes an exact interface for generating transactions to test on a given environment
// and a sized sequence of transactions to test on a given environment and reach a particular state
type TxWorkloadFactory interface {
	// NewWorkloads returns a set of TxWorkloadIterators from the VM. VM developers can use this function
	// to define each sequence of transactions that should be tested.
	// TODO: switch from workload generator to procedural test style for VM-defined workloads
	NewWorkloads(uri string) ([]TxWorkloadIterator, error)
	// Generates a new TxWorkloadIterator that generates a sequence of transactions of the given size.
	NewSizedTxWorkload(uri string, size int) (TxWorkloadIterator, error)

	// GetSpendingKey returns a private key to an account whcih the test case spend money from.
	GetSpendingKey() (*auth.PrivateKey, error)
}

type TxAssertion func(ctx context.Context, require *require.Assertions, uri string)

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
			confirm(ctx, require, uri)
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

func GenerateUntilStop(
	ctx context.Context,
	require *require.Assertions,
	uris []string,
	generator TxWorkloadIterator,
	stopChannel <-chan struct{},
) {
	submitClient := jsonrpc.NewJSONRPCClient(uris[0])
	for {
		select {
		case <-stopChannel:
			return
		default:
			if !generator.Next() {
				return
			}
			tx, confirm, err := generator.GenerateTxWithAssertion(ctx)
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
