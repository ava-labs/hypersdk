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
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/tests/fixture"
	"github.com/ava-labs/hypersdk/tests/workload"
)


var _ workload.TxGenerator = (*simpleTxWorkload)(nil)
var factory = &auth.ED25519Factory{}

// we should be able to pass this into the tx generator for its own use. hardcoded for now
func InitSimpleTx(key []*fixture.Ed25519TestKey) {
	factory = auth.NewED25519Factory(key[0].PrivKey)
}

const (
	TxCheckInterval = 100 * time.Millisecond
)


type simpleTxWorkload struct {
	count   int
	size    int
}

func NewTxGenerator(size int) workload.TxGenerator {
	return &simpleTxWorkload{
		size:    size,
	}
}

func (f *simpleTxWorkload) NewTxGenerator(size int) workload.TxGenerator {
	return &simpleTxWorkload{
		size:    size,
	}
}

func (g *simpleTxWorkload) GenerateTx(ctx context.Context, uri string) (*chain.Transaction, workload.TxAssertion, error) {
	cli := jsonrpc.NewJSONRPCClient(uri)
	lcli := vm.NewJSONRPCClient(uri)

	other, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return nil, nil, err
	}

	aother := auth.NewED25519Address(other.PublicKey())
	parser, err := lcli.Parser(ctx)
	if err != nil {
		return nil, nil, err
	}
	_, tx, _, err := cli.GenerateTransaction(
		ctx,
		parser,
		[]chain.Action{&actions.Transfer{
			To:    aother,
			Value: 1,
		}},
		factory,
	)
	if err != nil {
		return nil, nil, err
	}

	return tx, func(ctx context.Context, require *require.Assertions, uri string) {
		confirmTx(ctx, require, uri, tx.ID(), aother, 1)
	}, nil
}


func confirmTx(ctx context.Context, require *require.Assertions, uri string, txID ids.ID, receiverAddr codec.Address, receiverExpectedBalance uint64) {
	indexerCli := indexer.NewClient(uri)
	success, _, err := indexerCli.WaitForTransaction(ctx, TxCheckInterval, txID)
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
	transferOutputBytes := []byte(txRes.Outputs[0])
	require.Equal(consts.TransferID, transferOutputBytes[0])
	reader := codec.NewReader(transferOutputBytes, len(transferOutputBytes))
	transferOutputTyped, err := vm.OutputParser.Unmarshal(reader)
	require.NoError(err)
	transferOutput, ok := transferOutputTyped.(*actions.TransferResult)
	require.True(ok)
	require.Equal(receiverExpectedBalance, transferOutput.ReceiverBalance)
}
