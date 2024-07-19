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
	Owner codec.Address
	Metadata []byte
}

type Controller interface {
	Genesis() *genesis.Genesis
	Tracer() trace.Tracer
	GetTransaction(context.Context, ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error)
	GetBalanceFromState(context.Context, codec.Address) (uint64, error)
	GetNFTCollection(context.Context, codec.Address) (name []byte, symbol []byte, metadata []byte, numOfInstances uint32, collectionOwner codec.Address, err error)
	// GetAllNFTInstancesFromCollection(context.Context, codec.Address) ([]NFTInstanceDetails, error)
	GetNFTInstance(context.Context, codec.Address, uint32) (owner codec.Address, metadata []byte, err error)
	GetMarketplaceOrder(context.Context, ids.ID) (price uint64, err error)
}
