// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/enginetest"
	"github.com/ava-labs/avalanchego/snow/snowtest"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/snow"
	"github.com/ava-labs/hypersdk/state/balance"
	"github.com/ava-labs/hypersdk/state/metadata"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/stretchr/testify/require"
)

func NewTestVM(
	ctx context.Context,
	t *testing.T,
	chainID ids.ID,
) (*snow.VM[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock], *vm.VM) {
	r := require.New(t)
	var (
		actionParser = codec.NewTypeParser[chain.Action]()
		authParser   = codec.NewTypeParser[chain.Auth]()
		outputParser = codec.NewTypeParser[codec.Typed]()
	)
	r.NoError(errors.Join(
		actionParser.Register(&chaintest.TestAction{}, nil),
		authParser.Register(&auth.ED25519{}, auth.UnmarshalED25519),
		outputParser.Register(&chaintest.TestOutput{}, nil),
	))
	vm, err := vm.New(
		genesis.DefaultGenesisFactory{},
		balance.NewPrefixBalanceHandler([]byte{0}),
		metadata.NewDefaultManager(),
		actionParser,
		authParser,
		outputParser,
		auth.Engines(),
	)
	r.NoError(err)
	r.NotNil(vm)
	snowVM := snow.NewVM(vm)

	toEngine := make(chan common.Message, 1)
	genesis := genesis.DefaultGenesis{
		StateBranchFactor: merkledb.BranchFactor16,
		CustomAllocation:  []*genesis.CustomAllocation{},
		Rules:             genesis.NewDefaultRules(),
	}
	genesisBytes, err := json.Marshal(genesis)
	r.NoError(err)
	snowCtx := snowtest.Context(t, ids.GenerateTestID())
	snowCtx.ChainDataDir = t.TempDir()
	r.NoError(snowVM.Initialize(ctx, snowCtx, nil, genesisBytes, nil, nil, toEngine, nil, &enginetest.Sender{T: t}))
	return snowVM, vm
}

func TestSimpleVM(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	chainID := ids.GenerateTestID()
	snowVM1, vm1 := NewTestVM(ctx, t, chainID)
	snowVM2, vm2 := NewTestVM(ctx, t, chainID)

	// Create and submit tx, build block
	// Create a block
	block1, err := snowVM1.BuildBlock(ctx)
	r.NoError(err)
	r.NoError(block1.Verify(ctx))
	r.NoError(block1.Accept(ctx))

	block2, err := snowVM2.ParseBlock(ctx, block1.Bytes())
	r.NoError(err)
	r.NoError(block2.Verify(ctx))
	r.NoError(block2.Accept(ctx))

	_ = vm1
	_ = vm2
}

// build/parse correct sequence of blocks
// invalid block due due to auth
// invalid block due to action
// invalid due to time validity window (accepted window)
// invalid due to time validity window (processing ancestry)
// state sync
// submit tx and check gossip is handled correctly
// implement WorkloadNetwork
