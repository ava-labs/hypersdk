// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
	"github.com/ava-labs/hypersdk/examples/tokenvm/orderbook"
)

type Controller interface {
	Genesis() *genesis.Genesis
	Tracer() trace.Tracer
	GetTransaction(context.Context, ids.ID) (bool, int64, bool, chain.Dimensions, uint64, error)
	GetAssetFromState(context.Context, ids.ID) (bool, []byte, uint64, ed25519.PublicKey, bool, error)
	GetBalanceFromState(context.Context, ed25519.PublicKey, ids.ID) (uint64, error)
	Orders(pair string, limit int) []*orderbook.Order
	GetLoanFromState(context.Context, ids.ID, ids.ID) (uint64, error)
}
