// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/tests/e2e"
	"github.com/ava-labs/hypersdk/tests/workload"

	lrpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	ginkgo "github.com/onsi/ginkgo/v2"
)

var (
	_ workload.TxWorkloadFactory  = (*workloadFactory)(nil)
	_ workload.TxWorkloadIterator = (*simpleTxWorkload)(nil)
)

func TestE2e(t *testing.T) {
	ginkgo.RunSpecs(t, "morpheusvm e2e test suites")
}

func init() {
	require := require.New(ginkgo.GinkgoT())

	// Load default pk
	privBytes, err := codec.LoadHex(
		"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
		ed25519.PrivateKeyLen,
	)
	require.NoError(err)
	priv := ed25519.PrivateKey(privBytes)
	fmt.Println(priv.PublicKey())
	address := auth.NewED25519Address(priv.PublicKey())
	morpheusAddress, err := codec.AddressBech32(consts.HRP, address)
	require.NoError(err)
	factory := auth.NewED25519Factory(priv)

	gen := genesis.Default()
	gen.WindowTargetUnits = fees.Dimensions{40000000, 450000, 450000, 450000, 450000}
	gen.MaxBlockUnits = fees.Dimensions{1800000, 15000, 15000, 2500, 15000}
	gen.MinBlockGap = 100
	gen.CustomAllocation = []*genesis.CustomAllocation{
		{
			Address: morpheusAddress,
			Balance: 10000000000000000000,
		},
	}
	genesisBytes, err := json.Marshal(gen)
	require.NoError(err)

	e2e.SetupTestNetwork(
		consts.Name,
		consts.ID,
		genesisBytes,
		&workloadFactory{factory},
	)
}

type workloadFactory struct {
	factory *auth.ED25519Factory
}

func (f *workloadFactory) NewWorkloads(uri string) ([]workload.TxWorkloadIterator, error) {
	basicTxWorkload, err := f.NewSizedTxWorkload(uri, 1)
	return []workload.TxWorkloadIterator{basicTxWorkload}, err
}

func (f *workloadFactory) NewSizedTxWorkload(uri string, size int) (workload.TxWorkloadIterator, error) {
	cli := rpc.NewJSONRPCClient(uri)
	networkID, _, blockchainID, err := cli.Network(context.Background())
	if err != nil {
		return nil, err
	}
	lcli := lrpc.NewJSONRPCClient(uri, networkID, blockchainID)
	return &simpleTxWorkload{
		factory: f.factory,
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

func (g *simpleTxWorkload) GenerateTxWithAssertion(ctx context.Context) (*chain.Transaction, func(ctx context.Context, uri string) error, error) {
	g.count++
	// other, err := ed25519.GeneratePrivateKey()
	otherBytes, err := codec.LoadHex(
		"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
		ed25519.PrivateKeyLen,
	)
	if err != nil {
		return nil, nil, err
	}
	other := ed25519.PrivateKey(otherBytes)

	aother := auth.NewED25519Address(other.PublicKey())
	aotherStr := codec.MustAddressBech32(consts.HRP, aother)
	fmt.Println(aotherStr)
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
