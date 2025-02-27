// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package api

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state"
)

type VM interface {
	GetDataDir() string
	GetGenesisBytes() []byte
	Genesis() genesis.Genesis
	ChainID() ids.ID
	NetworkID() uint32
	SubnetID() ids.ID
	Tracer() trace.Tracer
	Logger() logging.Logger
	GetParser() chain.Parser
	GetABI() abi.ABI
	GetRuleFactory() chain.RuleFactory
	Submit(
		ctx context.Context,
		txs []*chain.Transaction,
	) (errs []error)
	LastAcceptedBlock(ctx context.Context) (*chain.StatelessBlock, error)
	UnitPrices(context.Context) (fees.Dimensions, error)
	CurrentValidators(
		context.Context,
	) (map[ids.NodeID]*validators.GetValidatorOutput, map[string]struct{})
	ReadState(ctx context.Context, keys [][]byte) ([][]byte, []error)
	ImmutableState(ctx context.Context) (state.Immutable, error)
	BalanceHandler() chain.BalanceHandler
}
