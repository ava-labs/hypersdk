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

type TxWorkloadFactory interface {
	NewBasicTxWorkload() TxWorkloadIterator
	NewSizedTxWorkload(size int) TxWorkloadIterator
	Workloads() []TxWorkloadIterator
}

type TxWorkloadIterator interface {
	Next() bool
	GenerateTxWithAssertion() (*chain.Transaction, func(ctx context.Context, uri string) error, error)
}

func ExecuteWorkload(ctx context.Context, require *require.Assertions, network Network, generator TxWorkloadIterator) {
	submitClient := rpc.NewJSONRPCClient(network.URIs()[0])

	for generator.Next() {
		tx, confirm, err := generator.GenerateTxWithAssertion()
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
	generator := factory.NewSizedTxWorkload(int(n))
	uri := network.URIs()[0]
	client := rpc.NewJSONRPCClient(uri)

	_, startHeight, _, err := client.Accepted(ctx)
	require.NoError(err)
	height := startHeight
	targetheight := startHeight + n

	for generator.Next() && height < targetheight {
		tx, confirm, err := generator.GenerateTxWithAssertion()
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
