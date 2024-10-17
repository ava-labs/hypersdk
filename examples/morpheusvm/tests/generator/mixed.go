// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package generator

import (
	"context"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/tests/fixture"
	"github.com/ava-labs/hypersdk/tests/workload"
)

var _ workload.TxGenerator = (*multiAuthTxGenerator)(nil)

type multiAuthTxGenerator struct {
	txCheckInterval     time.Duration
	addressAndFactories []addressAndFactory
	balance             uint64
	count               int
}

func NewMultiAuthTxGenerator(key *fixture.Ed25519TestKey, txCheckInterval time.Duration) (workload.TxGenerator, error) {
	blsPriv, err := bls.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	blsPub := bls.PublicFromPrivateKey(blsPriv)
	blsAddr := auth.NewBLSAddress(blsPub)
	blsFactory := auth.NewBLSFactory(blsPriv)

	secpPriv, err := secp256r1.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	secpPub := secpPriv.PublicKey()
	secpAddr := auth.NewSECP256R1Address(secpPub)
	secpFactory := auth.NewSECP256R1Factory(secpPriv)

	factories := []addressAndFactory{
		{address: key.Addr, authFactory: auth.NewED25519Factory(key.PrivKey)},
		{address: blsAddr, authFactory: blsFactory},
		{address: secpAddr, authFactory: secpFactory},
	}

	return &multiAuthTxGenerator{
		txCheckInterval:     txCheckInterval,
		addressAndFactories: factories,
		balance:             fixture.InitialBalance,
	}, nil
}

type addressAndFactory struct {
	address     codec.Address
	authFactory chain.AuthFactory
}

func (g *multiAuthTxGenerator) GenerateTx(ctx context.Context, uri string) (*chain.Transaction, workload.TxAssertion, error) {
	cli := jsonrpc.NewJSONRPCClient(uri)
	lcli := vm.NewJSONRPCClient(uri)
	defer func() { g.count++ }()
	if g.count >= len(g.addressAndFactories) {
		g.count = 0
	}
	sender := g.addressAndFactories[g.count]
	other, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return nil, nil, err
	}

	receiver := auth.NewED25519Address(other.PublicKey())

	parser, err := lcli.Parser(ctx)
	if err != nil {
		return nil, nil, err
	}
	_, tx, _, err := cli.GenerateTransaction(
		ctx,
		parser,
		[]chain.Action{&actions.Transfer{
			To:    receiver,
			Value: 1,
		}},
		sender.authFactory,
	)
	if err != nil {
		return nil, nil, err
	}

	return tx, func(ctx context.Context, require *require.Assertions, uri string) {
		confirmTx(ctx, require, uri, tx.ID(), receiver, 1, g.txCheckInterval)
	}, nil
}
