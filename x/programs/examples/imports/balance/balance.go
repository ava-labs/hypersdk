// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package balance

import (
	"context"

	"go.uber.org/zap"

	"github.com/bytecodealliance/wasmtime-go/v13"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

const Name = "balance"

func New(log logging.Logger, mu state.Mutable) *Import {
	return &Import{mu: mu, log: log}
}

type Import struct {
	mu    state.Mutable
	log   logging.Logger
	meter runtime.Meter
}

func (i *Import) Name() string {
	return Name
}

func (i *Import) Register(link runtime.Link, meter runtime.Meter, _ runtime.Imports) error {
	if err := link.FuncWrap(Name, "get", i.getFn); err != nil {
		return err
	}
	if err := link.FuncWrap(Name, "set", i.setFn); err != nil {
		return err
	}
	if err := link.FuncWrap(Name, "add", i.addFn); err != nil {
		return err
	}
	if err := link.FuncWrap(Name, "subtract", i.subtractFn); err != nil {
		return err
	}
	if err := link.FuncWrap(Name, "delete", i.delFn); err != nil {
		return err
	}

	i.meter = meter

	return nil
}

func (i *Import) getFn(caller *wasmtime.Caller, assetPtr int64, keyPtr int32) int32 {
	pk, err := runtime.PublicKeyFromOffset(caller, storage.HRP, keyPtr)
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return -1
	}

	assetID, err := runtime.IDFromOffset(caller, assetPtr)
	if err != nil {
		i.log.Error("failed to read assetID from memory",
			zap.Error(err),
		)
		return -1
	}

	// TODO: add cost for state access?
	// i.meter.Spend(BalanceCost)

	balance, err := storage.GetBalance(context.Background(), i.mu, pk)
	if err != nil {
		i.log.Error("failed to get balance",
			zap.Error(err),
		)
		return -1
	}

	return int32(balance)
}

func (i *Import) setFn(caller *wasmtime.Caller, assetPtr int64, keyPtr, balance int32) int32 {
	pk, err := runtime.PublicKeyFromOffset(caller, storage.HRP, keyPtr)
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return -1
	}
	assetID, err := runtime.IDFromOffset(caller, assetPtr)
	if err != nil {
		i.log.Error("failed to read assetID from memory",
			zap.Error(err),
		)
		return -1
	}

	// TODO: add cost for state access?
	// i.meter.Spend(BalanceCost)

	err = storage.SetBalance(context.Background(), i.mu, pk, assetID, uint64(balance))
	if err != nil {
		i.log.Error("failed to set balance",
			zap.Error(err),
		)
		return -1
	}

	return 0
}

func (i *Import) addFn(caller *wasmtime.Caller, assetPtr int64, keyPtr, amount int32) int32 {
	pk, err := runtime.PublicKeyFromOffset(caller, storage.HRP, keyPtr)
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return -1
	}
	assetID, err := runtime.IDFromOffset(caller, assetPtr)
	if err != nil {
		i.log.Error("failed to read assetID from memory",
			zap.Error(err),
		)
		return -1
	}

	// TODO: add cost for state access?
	// i.meter.Spend(BalanceCost)

	err = storage.AddBalance(context.Background(), i.mu, pk, assetID, uint64(amount), false)
	if err != nil {
		i.log.Error("failed to add balance",
			zap.Error(err),
		)
		return -1
	}

	return 0
}

func (i *Import) subtractFn(caller *wasmtime.Caller, assetPtr int64, keyPtr, amount int32) int32 {
	pk, err := runtime.PublicKeyFromOffset(caller, storage.HRP, keyPtr)
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return -1
	}
	assetID, err := runtime.IDFromOffset(caller, assetPtr)
	if err != nil {
		i.log.Error("failed to read assetID from memory",
			zap.Error(err),
		)
		return -1
	}

	// TODO: add cost for state access?
	// i.meter.Spend(BalanceCost)

	err = storage.SubBalance(context.Background(), i.mu, pk, assetID, uint64(amount))
	if err != nil {
		i.log.Error("failed to add balance",
			zap.Error(err),
		)
		return -1
	}

	return 0
}

func (i *Import) delFn(caller *wasmtime.Caller, assetPtr int64, keyPtr int32) int32 {
	pk, err := runtime.PublicKeyFromOffset(caller, storage.HRP, keyPtr)
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return -1
	}
	assetID, err := runtime.IDFromOffset(caller, assetPtr)
	if err != nil {
		i.log.Error("failed to read assetID from memory",
			zap.Error(err),
		)
		return -1
	}

	// TODO: add cost for state access?
	// i.meter.Spend(BalanceCost)

	err = storage.DeleteBalance(context.Background(), i.mu, pk, assetID)
	if err != nil {
		i.log.Error("failed to delete balance",
			zap.Error(err),
		)
		return -1
	}

	return 0
}
