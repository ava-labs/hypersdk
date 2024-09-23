// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/actions"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/consts"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"
	"github.com/ava-labs/hypersdk/extension/externalsubscriber"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/vm"
)

var (
	ActionParser *codec.TypeParser[chain.Action]
	AuthParser   *codec.TypeParser[chain.Auth]
	OutputParser *codec.TypeParser[codec.Typed]
)

// Setup types
func init() {
	ActionParser = codec.NewTypeParser[chain.Action]()
	AuthParser = codec.NewTypeParser[chain.Auth]()
	OutputParser = codec.NewTypeParser[codec.Typed]()

	errs := &wrappers.Errs{}
	errs.Add(
		// Token-related actions
		ActionParser.Register(&actions.CreateToken{}, nil),
		ActionParser.Register(&actions.MintToken{}, nil),
		ActionParser.Register(&actions.BurnToken{}, nil),
		ActionParser.Register(&actions.TransferToken{}, nil),

		// LP-related actions
		ActionParser.Register(&actions.CreateLiquidityPool{}, nil),
		ActionParser.Register(&actions.AddLiquidity{}, nil),
		ActionParser.Register(&actions.RemoveLiquidity{}, nil),
		ActionParser.Register(&actions.Swap{}, nil),

		AuthParser.Register(&auth.ED25519{}, auth.UnmarshalED25519),
		AuthParser.Register(&auth.SECP256R1{}, auth.UnmarshalSECP256R1),
		AuthParser.Register(&auth.BLS{}, auth.UnmarshalBLS),

		OutputParser.Register(&actions.CreateTokenResult{}, nil),
		OutputParser.Register(&actions.MintTokenResult{}, nil),
		OutputParser.Register(&actions.BurnTokenResult{}, nil),
		OutputParser.Register(&actions.TransferTokenResult{}, nil),
		OutputParser.Register(&actions.CreateLiquidityPoolResult{}, nil),
		OutputParser.Register(&actions.AddLiquidityResult{}, nil),
		OutputParser.Register(&actions.RemoveLiquidityResult{}, nil),
		OutputParser.Register(&actions.SwapResult{}, nil),

		// Read-only actions
		ActionParser.Register(&actions.GetTokenInfo{}, nil),
		ActionParser.Register(&actions.GetTokenAccountBalance{}, nil),
		ActionParser.Register(&actions.GetLiquidityPoolInfo{}, nil),

		OutputParser.Register(&actions.GetTokenInfoResult{}, nil),
		OutputParser.Register(&actions.GetTokenAccountBalanceResult{}, nil),
		OutputParser.Register(&actions.GetLiquidityPoolInfoResult{}, nil),
	)
	if errs.Errored() {
		panic(errs.Err)
	}
}

// New returns a VM with the indexer, websocket, rpc, and external subscriber apis enabled.
func New(options ...vm.Option) (*vm.VM, error) {
	opts := append([]vm.Option{
		indexer.With(),
		ws.With(),
		jsonrpc.With(),
		// TODO: reimplement Controller API
		With(), // Add Controller API
		externalsubscriber.With(),
	}, options...)

	return NewWithOptions(opts...)
}

// NewWithOptions returns a VM with the specified options
func NewWithOptions(options ...vm.Option) (*vm.VM, error) {
	return vm.New(
		consts.Version,
		genesis.DefaultGenesisFactory{},
		&storage.StateManager{},
		ActionParser,
		AuthParser,
		OutputParser,
		auth.Engines(),
		options...,
	)
}
