// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
	"github.com/ava-labs/hypersdk/examples/tokenvm/orderbook"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/fees"
)

func (vm *VM) Genesis() *genesis.Genesis {
	return vm.genesis
}

func (vm *VM) Logger() logging.Logger {
	return vm.inner.Logger()
}

func (vm *VM) Tracer() trace.Tracer {
	return vm.inner.Tracer()
}

func (vm *VM) GetTransaction(
	ctx context.Context,
	txID ids.ID,
) (bool, int64, bool, fees.Dimensions, uint64, error) {
	return storage.GetTransaction(ctx, vm.db, txID)
}

func (vm *VM) GetAssetFromState(
	ctx context.Context,
	asset ids.ID,
) (bool, []byte, uint8, []byte, uint64, codec.Address, error) {
	return storage.GetAssetFromState(ctx, vm.inner.ReadState, asset)
}

func (vm *VM) GetBalanceFromState(
	ctx context.Context,
	addr codec.Address,
	asset ids.ID,
) (uint64, error) {
	return storage.GetBalanceFromState(ctx, vm.inner.ReadState, addr, asset)
}

func (vm *VM) Orders(pair string, limit int) []*orderbook.Order {
	return vm.orderBook.Orders(pair, limit)
}

func (vm *VM) GetOrderFromState(
	ctx context.Context,
	orderID ids.ID,
) (
	bool, // exists
	ids.ID, // in
	uint64, // inTick
	ids.ID, // out
	uint64, // outTick
	uint64, // remaining
	codec.Address, // owner
	error,
) {
	return storage.GetOrderFromState(ctx, vm.inner.ReadState, orderID)
}
