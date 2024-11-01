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
	"github.com/ava-labs/hypersdk/examples/testvm/actions"
	"github.com/ava-labs/hypersdk/examples/testvm/vm"
	"github.com/ava-labs/hypersdk/tests/workload"
)

var _ workload.TxGenerator = (*TxGenerator)(nil)

const txCheckInterval = 100 * time.Millisecond

type TxGenerator struct {
	factory *auth.ED25519Factory
}

func NewTxGenerator(key ed25519.PrivateKey) *TxGenerator {
	return &TxGenerator{
		factory: auth.NewED25519Factory(key),
	}
}

func (g *TxGenerator) GenerateTx(ctx context.Context, uri string) (*chain.Transaction, workload.TxAssertion, error) {
	cli := jsonrpc.NewJSONRPCClient(uri)
	lcli := vm.NewJSONRPCClient(uri)

	privateKey, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return nil, nil, err
	}
	address := auth.NewED25519Address(privateKey.PublicKey())
	incAmount := uint64(1)
	parser, err := lcli.Parser(ctx)
	if err != nil {
		return nil, nil, err
	}
	_, tx, _, err := cli.GenerateTransaction(
		ctx,
		parser,
		[]chain.Action{&actions.Count{
			Amount: incAmount,
			Address: address,
		}},
		g.factory,
	)
	if err != nil {
		return nil, nil, err
	}

	return tx, func(ctx context.Context, require *require.Assertions, uri string) {
		confirmTx(ctx, require, uri, tx.ID(), address, incAmount)
	}, nil
}

func confirmTx(ctx context.Context, require *require.Assertions, uri string, txID ids.ID, addr codec.Address, expectedCount uint64) {

	indexerCli := indexer.NewClient(uri)
	success, _, err := indexerCli.WaitForTransaction(ctx, txCheckInterval, txID)
	require.NoError(err)
	require.True(success)
	// lcli := vm.NewJSONRPCClient(uri)
	// balance, err := lcli.Count(ctx, addr)
	// require.NoError(err)
	// require.Equal(expectedCount, balance)
	// txRes, _, err := indexerCli.GetTx(ctx, txID)
	// require.NoError(err)
	// // TODO: perform exact expected fee, units check, and output check
	// require.NotZero(txRes.Fee)
	// require.Len(txRes.Outputs, 1)
	// countOutputBytes := []byte(txRes.Outputs[0])
	// require.Equal(consts.CountID, countOutputBytes[0])
	// reader := codec.NewReader(countOutputBytes, len(countOutputBytes))
	// countOutputTyped, err := vm.OutputParser.Unmarshal(reader)
	// require.NoError(err)
	// transferOutput, ok := countOutputTyped.(*actions.CountResult)
	// require.True(ok)
	// require.Equal(expectedCount, transferOutput.Count)
	
}
