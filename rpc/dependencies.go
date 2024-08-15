// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/anchor"
	"github.com/ava-labs/hypersdk/chain"
)

type VM interface {
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
	LastAcceptedBlock() *chain.StatelessBlock
	UnitPrices(context.Context) (chain.Dimensions, error)
	GetOutgoingWarpMessage(ids.ID) (*warp.UnsignedMessage, error)
	GetWarpSignatures(ids.ID) ([]*chain.WarpSignature, error)
	IterateCurrentValidators(context.Context, func(ids.NodeID, *validators.GetValidatorOutput)) error
	GatherSignatures(context.Context, ids.ID, []byte)

	GetVerifyAuth() bool
	GetAuthRPCCores() int
	GetAuthRPCBacklog() int
	RecordRPCTxBacklog(int64)
	AddRPCAuthorized(tx *chain.Transaction)
	StopChan() chan struct{}

	RecordWebsocketConnection(int)
	RecordRPCTxInvalid()

	Anchor() *anchor.Anchor
}
