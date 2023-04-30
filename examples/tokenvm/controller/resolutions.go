// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
	"github.com/ava-labs/hypersdk/examples/tokenvm/orderbook"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
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
) (bool, int64, bool, uint64, error) {
	return storage.GetTransaction(ctx, c.metaDB, txID)
}

func (c *Controller) GetAssetFromState(
	ctx context.Context,
	asset ids.ID,
) (bool, []byte, uint64, crypto.PublicKey, bool, error) {
	return storage.GetAssetFromState(ctx, c.inner.ReadState, asset)
}

func (c *Controller) GetBalanceFromState(
	ctx context.Context,
	pk crypto.PublicKey,
	asset ids.ID,
) (uint64, error) {
	return storage.GetBalanceFromState(ctx, c.inner.ReadState, pk, asset)
}

func (c *Controller) Orders(pair string, limit int) []*orderbook.Order {
	return c.orderBook.Orders(pair, limit)
}

func (c *Controller) GetLoanFromState(
	ctx context.Context,
	asset ids.ID,
	destination ids.ID,
) (uint64, error) {
	return storage.GetLoanFromState(ctx, c.inner.ReadState, asset, destination)
}
