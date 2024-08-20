// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/tests/workload"

	lrpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
)

const initialBalance uint64 = 10_000_000_000_000

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
	ed25519AddrStrs      = make([]string, len(ed25519HexKeys))
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
		ed25519AddrStrs[i] = codec.MustAddressBech32(consts.HRP, addr)
	}
}

type workloadFactory struct {
	factories []*auth.ED25519Factory
	addrs     []codec.Address
}

func New(minBlockGap int64) (*genesis.Genesis, workload.TxWorkloadFactory, error) {
	gen := genesis.Default()
	// Set WindowTargetUnits to MaxUint64 for all dimensions to iterate full mempool during block building.
	// gen.WindowTargetUnits = fees.Dimensions{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	// // Set all lmiits to MaxUint64 to avoid limiting block size for all dimensions except bandwidth. Must limit bandwidth to avoid building
	// // a block that exceeds the maximum size allowed by AvalancheGo.
	// gen.MaxBlockUnits = fees.Dimensions{1800000, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	// gen.MinBlockGap = minBlockGap

	gen.StateBranchFactor = 16
	gen.MinBlockGap = 50
	gen.MinEmptyBlockGap = 2500
	gen.MinUnitPrice = fees.Dimensions{100, 100, 100, 100, 100}
	gen.UnitPriceChangeDenominator = fees.Dimensions{48, 48, 48, 48, 48}
	gen.WindowTargetUnits = fees.Dimensions{20000000, 20000, 20000, 20000, 20000}
	gen.MaxBlockUnits = fees.Dimensions{1800000, 2000, 2000, 2000, 2000}
	gen.ValidityWindow = 60000
	gen.MaxActionsPerTx = 16
	gen.MaxOutputsPerAction = 1
	gen.BaseComputeUnits = 1
	gen.StorageKeyReadUnits = 0
	gen.StorageValueReadUnits = 0
	gen.StorageKeyAllocateUnits = 10
	gen.StorageValueAllocateUnits = 0
	gen.StorageKeyWriteUnits = 1
	gen.StorageValueWriteUnits = 1

	// for _, prefundedAddrStr := range ed25519AddrStrs {
	// 	gen.CustomAllocation = append(gen.CustomAllocation, &genesis.CustomAllocation{
	// 		Address: prefundedAddrStr,
	// 		Balance: initialBalance,
	// 	})
	// }

	gen.CustomAllocation = []*genesis.CustomAllocation{
		&genesis.CustomAllocation{
			// Address: "morph1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdj73w34s",
			Address: "morpheus1q8rc050907hx39vfejpawjydmwe6uujw0njx9s6skzdpp3cm2he5s036p07",
			Balance: 10000000000000000000,
		},
	}

	return gen, &workloadFactory{
		factories: ed25519AuthFactories,
		addrs:     ed25519Addrs,
	}, nil
}

func (f *workloadFactory) NewSizedTxWorkload(uri string, size int) (workload.TxWorkloadIterator, error) {
	cli := rpc.NewJSONRPCClient(uri)
	networkID, _, blockchainID, err := cli.Network(context.Background())
	if err != nil {
		return nil, err
	}
	lcli := lrpc.NewJSONRPCClient(uri, networkID, blockchainID)
	return &simpleTxWorkload{
		factory: f.factories[0],
		cli:     cli,
		lcli:    lcli,
		size:    size,
	}, nil
}

type simpleTxWorkload struct {
	factory   *auth.ED25519Factory
	cli       *rpc.JSONRPCClient
	lcli      *lrpc.JSONRPCClient
	networkID uint32
	chainID   ids.ID
	count     int
	size      int
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
	aotherStr := codec.MustAddressBech32(consts.HRP, aother)
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

	return tx, func(ctx context.Context, uri string) error {
		lcli := lrpc.NewJSONRPCClient(uri, g.networkID, g.chainID)
		success, _, err := lcli.WaitForTransaction(ctx, tx.ID())
		if err != nil {
			return fmt.Errorf("failed to wait for tx %s: %w", tx.ID(), err)
		}
		if !success {
			return fmt.Errorf("tx %s not accepted", tx.ID())
		}
		balance, err := lcli.Balance(ctx, aotherStr)
		if err != nil {
			return fmt.Errorf("failed to get balance of %s: %w", aotherStr, err)
		}
		if balance != 1 {
			return fmt.Errorf("expected balance of 1, got %d", balance)
		}
		return nil
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

	cli := rpc.NewJSONRPCClient(uri)
	networkID, _, blockchainID, err := cli.Network(context.Background())
	if err != nil {
		return nil, err
	}
	lcli := lrpc.NewJSONRPCClient(uri, networkID, blockchainID)

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
	cli                 *rpc.JSONRPCClient
	lcli                *lrpc.JSONRPCClient
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
	receiverAddrStr := codec.MustAddressBech32(consts.HRP, receiver.address)
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

	return tx, func(ctx context.Context, uri string) error {
		lcli := lrpc.NewJSONRPCClient(uri, g.networkID, g.chainID)
		success, _, err := lcli.WaitForTransaction(ctx, tx.ID())
		if err != nil {
			return fmt.Errorf("failed to wait for tx %s: %w", tx.ID(), err)
		}
		if !success {
			return fmt.Errorf("tx %s not accepted", tx.ID())
		}
		balance, err := lcli.Balance(ctx, receiverAddrStr)
		if err != nil {
			return fmt.Errorf("failed to get balance of %s: %w", receiverAddrStr, err)
		}
		if balance != expectedBalance {
			return fmt.Errorf("expected balance of 1, got %d", balance)
		}
		// TODO check tx fee + units (not currently available via API)
		return nil
	}, nil
}
