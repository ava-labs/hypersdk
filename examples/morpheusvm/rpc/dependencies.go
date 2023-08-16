// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
)

type Controller interface {
	Genesis() *genesis.Genesis
	Tracer() trace.Tracer
	GetTransaction(context.Context, ids.ID) (bool, int64, bool, chain.Dimensions, uint64, error)
	GetBalanceFromState(context.Context, ed25519.PublicKey) (uint64, error)
}
