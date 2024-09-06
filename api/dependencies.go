// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package api

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/fees"
)

type VM interface {
	Genesis() genesis.Genesis
	ChainID() ids.ID
	NetworkID() uint32
	SubnetID() ids.ID
	Tracer() trace.Tracer
	Logger() logging.Logger
	Registry() (chain.ActionRegistry, chain.AuthRegistry)
	Submit(
		ctx context.Context,
		verifySig bool,
		txs []*chain.Transaction,
	) (errs []error)
	LastAcceptedBlock() *chain.StatefulBlock
	UnitPrices(context.Context) (fees.Dimensions, error)
	CurrentValidators(
		context.Context,
	) (map[ids.NodeID]*validators.GetValidatorOutput, map[string]struct{})
	GetVerifyAuth() bool
	ReadState(ctx context.Context, keys [][]byte) ([][]byte, []error)
}
