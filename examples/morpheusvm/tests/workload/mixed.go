// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

// import (
// 	"context"

// 	"github.com/ava-labs/hypersdk/api/jsonrpc"
// 	"github.com/ava-labs/hypersdk/auth"
// 	"github.com/ava-labs/hypersdk/chain"
// 	"github.com/ava-labs/hypersdk/codec"
// 	"github.com/ava-labs/hypersdk/crypto/bls"
// 	"github.com/ava-labs/hypersdk/crypto/secp256r1"
// 	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
// 	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
// 	"github.com/ava-labs/hypersdk/tests/fixture"
// 	"github.com/ava-labs/hypersdk/tests/workload"
// 	"github.com/stretchr/testify/require"
// )

// func (f *workloadFactory) NewWorkloads(uri string) ([]workload.TxWorkloadIterator, error) {
// 	blsPriv, err := bls.GeneratePrivateKey()
// 	if err != nil {
// 		return nil, err
// 	}
// 	blsPub := bls.PublicFromPrivateKey(blsPriv)
// 	blsAddr := auth.NewBLSAddress(blsPub)
// 	blsFactory := auth.NewBLSFactory(blsPriv)

// 	secpPriv, err := secp256r1.GeneratePrivateKey()
// 	if err != nil {
// 		return nil, err
// 	}
// 	secpPub := secpPriv.PublicKey()
// 	secpAddr := auth.NewSECP256R1Address(secpPub)
// 	secpFactory := auth.NewSECP256R1Factory(secpPriv)

// 	cli := jsonrpc.NewJSONRPCClient(uri)
// 	lcli := vm.NewJSONRPCClient(uri)

// 	generator := &mixedAuthWorkload{
// 		addressAndFactories: []addressAndFactory{
// 			{address: f.keys[1].Addr, authFactory: f.keys[1].AuthFactory},
// 			{address: blsAddr, authFactory: blsFactory},
// 			{address: secpAddr, authFactory: secpFactory},
// 		},
// 		balance:   fixture.InitialBalance,
// 		cli:       cli,
// 		lcli:      lcli,
// 	}

// 	return []workload.TxWorkloadIterator{generator}, nil
// }

// type addressAndFactory struct {
// 	address     codec.Address
// 	authFactory chain.AuthFactory
// }

// type mixedAuthWorkload struct {
// 	addressAndFactories []addressAndFactory
// 	balance             uint64
// 	cli                 *jsonrpc.JSONRPCClient
// 	lcli                *vm.JSONRPCClient
// 	count               int
// }

// func (g *mixedAuthWorkload) Next() bool {
// 	return g.count < len(g.addressAndFactories)-1
// }

// func (g *mixedAuthWorkload) GenerateTxWithAssertion(ctx context.Context) (*chain.Transaction, workload.TxAssertion, error) {
// 	defer func() { g.count++ }()

// 	sender := g.addressAndFactories[g.count]
// 	receiver := g.addressAndFactories[g.count+1]
// 	expectedBalance := g.balance - 1_000_000

// 	parser, err := g.lcli.Parser(ctx)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	_, tx, _, err := g.cli.GenerateTransaction(
// 		ctx,
// 		parser,
// 		[]chain.Action{&actions.Transfer{
// 			To:    receiver.address,
// 			Value: expectedBalance,
// 		}},
// 		sender.authFactory,
// 	)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	g.balance = expectedBalance

// 	return tx, func(ctx context.Context, require *require.Assertions, uri string) {
// 		confirmTx(ctx, require, uri, tx.ID(), receiver.address, expectedBalance)
// 	}, nil
// }
