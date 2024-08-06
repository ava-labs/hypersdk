// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/nftvm/genesis"
	"github.com/ava-labs/hypersdk/examples/nftvm/storage"
	"github.com/ava-labs/hypersdk/fees"
)

func (c *Controller) Genesis() *genesis.Genesis {
	return c.genesis
}

func (c *Controller) Logger() logging.Logger {
	return c.inner.Logger()
}

func (c *Controller) Tracer() trace.Tracer {
	return c.inner.Tracer()
}

func (c *Controller) GetTransaction(
	ctx context.Context,
	txID ids.ID,
) (bool, int64, bool, fees.Dimensions, uint64, error) {
	return storage.GetTransaction(ctx, c.metaDB, txID)
}

func (c *Controller) GetBalanceFromState(
	ctx context.Context,
	acct codec.Address,
) (uint64, error) {
	return storage.GetBalanceFromState(ctx, c.inner.ReadState, acct)
}

func (c *Controller) GetNFTCollection(
	ctx context.Context,
	collectionAddress codec.Address,
) ([]byte, []byte, []byte, uint32, codec.Address, error) {
	return storage.GetNFTCollection(ctx, c.inner.ReadState, collectionAddress)
}

func (c *Controller) GetNFTInstance(
	ctx context.Context,
	collectionAddress codec.Address,
	instanceNum uint32,
) (codec.Address, []byte, bool, error) {
	return storage.GetNFTInstance(ctx, c.inner.ReadState, collectionAddress, instanceNum)
}

func (c *Controller) GetMarketplaceOrder(
	ctx context.Context,
	orderID ids.ID,
) (uint64, error) {
	return storage.GetMarketplaceOrder(ctx, c.inner.ReadState, orderID)
}
