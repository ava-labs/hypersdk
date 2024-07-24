// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"

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
