// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/nftvm/genesis"
	"github.com/ava-labs/hypersdk/fees"
)

type NFTInstanceDetails struct {
	Owner    codec.Address
	Metadata []byte
}

type Controller interface {
	Genesis() *genesis.Genesis
	Tracer() trace.Tracer
	GetTransaction(context.Context, ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error)
	GetBalanceFromState(context.Context, codec.Address) (uint64, error)
	GetNFTCollection(context.Context, codec.Address) ([]byte, []byte, []byte, uint32, codec.Address, error)
	GetNFTInstance(context.Context, codec.Address, uint32) (codec.Address, []byte, bool, error)
	GetMarketplaceOrder(context.Context, ids.ID) (uint64, error)
}
