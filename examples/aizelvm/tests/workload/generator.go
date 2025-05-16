// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/aizelvm/actions"
	"github.com/ava-labs/hypersdk/examples/aizelvm/consts"
	"github.com/ava-labs/hypersdk/examples/aizelvm/vm"
	"github.com/ava-labs/hypersdk/tests/workload"
)

var _ workload.TxGenerator = (*TxGenerator)(nil)

const txCheckInterval = 100 * time.Millisecond

type TxGenerator struct {
	factory chain.AuthFactory
}

func NewTxGenerator(authFactory chain.AuthFactory) *TxGenerator {
	return &TxGenerator{
		factory: authFactory,
	}
}

func (g *TxGenerator) GenerateTx(ctx context.Context, uri string) (*chain.Transaction, workload.TxAssertion, error) {
	// TODO: no need to generate the clients every tx
	cli := jsonrpc.NewJSONRPCClient(uri)
	lcli := vm.NewJSONRPCClient(uri)
	to, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return nil, nil, err
	}
	ruleFactory, err := lcli.GetRuleFactory(ctx)
	if err != nil {
		return nil, nil, err
	}

	toAddress := auth.NewED25519Address(to.PublicKey())

	unitPrices, err := cli.UnitPrices(ctx, true)
	if err != nil {
		return nil, nil, err
	}

	tx, err := chain.GenerateTransaction(
		ruleFactory,
		unitPrices,
		time.Now().UnixMilli(),
		[]chain.Action{&actions.Transfer{
			To:    toAddress,
			Value: 1,
		}},
		g.factory,
	)
	if err != nil {
		return nil, nil, err
	}

	return tx, func(ctx context.Context, require *require.Assertions, uri string) {
		confirmTx(ctx, require, uri, tx.GetID(), toAddress, 1)
	}, nil
}

func confirmTx(ctx context.Context, require *require.Assertions, uri string, txID ids.ID, receiverAddr codec.Address, receiverExpectedBalance uint64) {
	lcli := vm.NewJSONRPCClient(uri)
	parser := lcli.GetParser()
	indexerCli := indexer.NewClient(uri)
	success, _, err := indexerCli.WaitForTransaction(ctx, txCheckInterval, txID)
	require.NoError(err)
	require.True(success)
	balance, err := lcli.Balance(ctx, receiverAddr)
	require.NoError(err)
	require.Equal(receiverExpectedBalance, balance)
	txRes, _, _, err := indexerCli.GetTx(ctx, txID, parser)
	require.NoError(err)
	// TODO: perform exact expected fee, units check, and output check
	require.NotZero(txRes.Result.Fee)
	require.Len(txRes.Result.Outputs, 1)
	transferOutputBytes := txRes.Result.Outputs[0]
	require.Equal(consts.TransferID, transferOutputBytes[0])
	transferOutputTyped, err := vm.OutputParser.Unmarshal(transferOutputBytes)
	require.NoError(err)
	transferOutput, ok := transferOutputTyped.(*actions.TransferResult)
	require.True(ok)
	require.Equal(receiverExpectedBalance, transferOutput.ReceiverBalance)
}
