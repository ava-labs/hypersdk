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
	"github.com/ava-labs/hypersdk/examples/hyperevm/actions"
	"github.com/ava-labs/hypersdk/examples/hyperevm/consts"
	"github.com/ava-labs/hypersdk/examples/hyperevm/storage"
	"github.com/ava-labs/hypersdk/examples/hyperevm/vm"
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

	fromAddress := storage.ToEVMAddress(g.factory.Address())

	action := &actions.EvmCall{
		To:       storage.ToEVMAddress(toAddress),
		Value:    1,
		GasLimit: 21_000,
		From:     fromAddress,
	}

	result, err := lcli.SimulateActions(ctx, []chain.Action{action}, g.factory.Address())
	if err != nil {
		return nil, nil, err
	}
	action.Keys = result[0].StateKeys

	tx, err := chain.GenerateTransaction(
		ruleFactory,
		unitPrices,
		[]chain.Action{action},
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
	indexerCli := indexer.NewClient(uri)
	success, _, err := indexerCli.WaitForTransaction(ctx, txCheckInterval, txID)
	require.NoError(err)
	require.True(success)
	lcli := vm.NewJSONRPCClient(uri)
	balance, err := lcli.Balance(ctx, receiverAddr)
	require.NoError(err)
	require.Equal(receiverExpectedBalance, balance)
	txRes, _, err := indexerCli.GetTx(ctx, txID)
	require.NoError(err)
	// TODO: perform exact expected fee, units check, and output check
	require.NotZero(txRes.Fee)
	require.Len(txRes.Outputs, 1)
	evmCallOutputBytes := []byte(txRes.Outputs[0])
	require.Equal(consts.EvmCallID, evmCallOutputBytes[0])
	transferOutputTyped, err := vm.OutputParser.Unmarshal(evmCallOutputBytes)
	require.NoError(err)
	transferOutput, ok := transferOutputTyped.(*actions.EvmCallResult)
	require.True(ok)
	require.True(transferOutput.Success)
	require.Equal(uint64(21_000), transferOutput.UsedGas)
}
