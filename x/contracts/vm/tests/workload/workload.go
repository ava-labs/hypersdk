// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"
	"math"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/tests/workload"
	"github.com/ava-labs/hypersdk/x/contracts/vm/actions"
	"github.com/ava-labs/hypersdk/x/contracts/vm/vm"
)

const (
	initialBalance  uint64 = 10_000_000_000_000
	txCheckInterval        = 100 * time.Millisecond
)

var (
	_              workload.TxWorkloadFactory  = (*workloadFactory)(nil)
	_              workload.TxWorkloadIterator = (*simpleTxWorkload)(nil)
	ed25519HexKeys                             = []string{
		"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
		"8a7be2e0c9a2d09ac2861c34326d6fe5a461d920ba9c2b345ae28e603d517df148735063f8d5d8ba79ea4668358943e5c80bc09e9b2b9a15b5b15db6c1862e88", //nolint:lll
	}
	ed25519PrivKeys      = make([]ed25519.PrivateKey, len(ed25519HexKeys))
	ed25519Addrs         = make([]codec.Address, len(ed25519HexKeys))
	ed25519AuthFactories = make([]*auth.ED25519Factory, len(ed25519HexKeys))
)

func init() {
	for i, keyHex := range ed25519HexKeys {
		privBytes, err := codec.LoadHex(keyHex, ed25519.PrivateKeyLen)
		if err != nil {
			panic(err)
		}
		priv := ed25519.PrivateKey(privBytes)
		ed25519PrivKeys[i] = priv
		ed25519AuthFactories[i] = auth.NewED25519Factory(priv)
		addr := auth.NewED25519Address(priv.PublicKey())
		ed25519Addrs[i] = addr
	}
}

type workloadFactory struct {
	factories []*auth.ED25519Factory
	addrs     []codec.Address
}

func New(minBlockGap int64) (*genesis.DefaultGenesis, workload.TxWorkloadFactory, error) {
	customAllocs := make([]*genesis.CustomAllocation, 0, len(ed25519Addrs))
	for _, prefundedAddrStr := range ed25519Addrs {
		customAllocs = append(customAllocs, &genesis.CustomAllocation{
			Address: prefundedAddrStr,
			Balance: initialBalance,
		})
	}

	genesis := genesis.NewDefaultGenesis(customAllocs)
	// Set WindowTargetUnits to MaxUint64 for all dimensions to iterate full mempool during block building.
	genesis.Rules.WindowTargetUnits = fees.Dimensions{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	// Set all limits to MaxUint64 to avoid limiting block size for all dimensions except bandwidth. Must limit bandwidth to avoid building
	// a block that exceeds the maximum size allowed by AvalancheGo.
	genesis.Rules.MaxBlockUnits = fees.Dimensions{1800000, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	genesis.Rules.MinBlockGap = minBlockGap

	return genesis, &workloadFactory{
		factories: ed25519AuthFactories,
		addrs:     ed25519Addrs,
	}, nil
}

func (*workloadFactory) GetSpendingKey() (*auth.PrivateKey, error) {
	return &auth.PrivateKey{
		Address: ed25519Addrs[0],
		Bytes:   ed25519PrivKeys[0][:],
	}, nil
}

func (f *workloadFactory) NewSizedTxWorkload(uri string, size int) (workload.TxWorkloadIterator, error) {
	cli := jsonrpc.NewJSONRPCClient(uri)
	lcli := vm.NewJSONRPCClient(uri)
	return &simpleTxWorkload{
		factory: f.factories[0],
		cli:     cli,
		lcli:    lcli,
		size:    size,
	}, nil
}

type simpleTxWorkload struct {
	factory *auth.ED25519Factory
	cli     *jsonrpc.JSONRPCClient
	lcli    *vm.JSONRPCClient
	count   int
	size    int
}

func (g *simpleTxWorkload) Next() bool {
	return g.count < g.size
}

func (g *simpleTxWorkload) GenerateTxWithAssertion(ctx context.Context) (*chain.Transaction, workload.TxAssertion, error) {
	g.count++
	other, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return nil, nil, err
	}

	aother := auth.NewED25519Address(other.PublicKey())
	parser, err := g.lcli.Parser(ctx)
	if err != nil {
		return nil, nil, err
	}
	_, tx, _, err := g.cli.GenerateTransaction(
		ctx,
		parser,
		[]chain.Action{&actions.Transfer{
			To:    aother,
			Value: 1,
		}},
		g.factory,
	)
	if err != nil {
		return nil, nil, err
	}

	return tx, func(ctx context.Context, require *require.Assertions, uri string) {
		indexerCli := indexer.NewClient(uri)
		success, _, err := indexerCli.WaitForTransaction(ctx, txCheckInterval, tx.ID())
		require.NoError(err)
		require.True(success)
		lcli := vm.NewJSONRPCClient(uri)
		balance, err := lcli.Balance(ctx, aother)
		require.NoError(err)
		require.Equal(uint64(1), balance)
	}, nil
}

func (f *workloadFactory) NewWorkloads(uri string) ([]workload.TxWorkloadIterator, error) {
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

	cli := jsonrpc.NewJSONRPCClient(uri)
	networkID, _, blockchainID, err := cli.Network(context.Background())
	if err != nil {
		return nil, err
	}
	lcli := vm.NewJSONRPCClient(uri)

	generator := &mixedAuthWorkload{
		addressAndFactories: []addressAndFactory{
			{address: f.addrs[1], authFactory: f.factories[1]},
			{address: blsAddr, authFactory: blsFactory},
			{address: secpAddr, authFactory: secpFactory},
		},
		balance:   initialBalance,
		cli:       cli,
		lcli:      lcli,
		networkID: networkID,
		chainID:   blockchainID,
	}

	return []workload.TxWorkloadIterator{generator}, nil
}

type addressAndFactory struct {
	address     codec.Address
	authFactory chain.AuthFactory
}

type mixedAuthWorkload struct {
	addressAndFactories []addressAndFactory
	balance             uint64
	cli                 *jsonrpc.JSONRPCClient
	lcli                *vm.JSONRPCClient
	networkID           uint32
	chainID             ids.ID
	count               int
}

func (g *mixedAuthWorkload) Next() bool {
	return g.count < len(g.addressAndFactories)-1
}

func (g *mixedAuthWorkload) GenerateTxWithAssertion(ctx context.Context) (*chain.Transaction, workload.TxAssertion, error) {
	defer func() { g.count++ }()

	sender := g.addressAndFactories[g.count]
	receiver := g.addressAndFactories[g.count+1]
	expectedBalance := g.balance - 1_000_000

	parser, err := g.lcli.Parser(ctx)
	if err != nil {
		return nil, nil, err
	}
	_, tx, _, err := g.cli.GenerateTransaction(
		ctx,
		parser,
		[]chain.Action{&actions.Transfer{
			To:    receiver.address,
			Value: expectedBalance,
		}},
		sender.authFactory,
	)
	if err != nil {
		return nil, nil, err
	}
	g.balance = expectedBalance

	return tx, func(ctx context.Context, require *require.Assertions, uri string) {
		indexerCli := indexer.NewClient(uri)
		success, _, err := indexerCli.WaitForTransaction(ctx, txCheckInterval, tx.ID())
		require.NoError(err)
		require.True(success)
		lcli := vm.NewJSONRPCClient(uri)
		balance, err := lcli.Balance(ctx, receiver.address)
		require.NoError(err)
		require.Equal(expectedBalance, balance)
		// TODO check tx fee + units (not currently available via API)
	}, nil
}
