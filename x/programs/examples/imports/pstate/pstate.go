// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pstate

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/wrap"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/program"
	"github.com/ava-labs/hypersdk/x/programs/program/types"
)

var _ host.Import = (*Import)(nil)

const Name = "state"

// New returns a program storage module capable of storing arbitrary bytes
// in the program's namespace.
func New(log logging.Logger, mu state.Mutable) host.Import {
	return &Import{mu: mu, log: log}
}

type Import struct {
	mu    state.Mutable
	log   logging.Logger
	meter *engine.Meter
}

func (i *Import) Name() string {
	return Name
}

func (i *Import) Register(link *host.Link) error {
	i.meter = link.Meter()
	wrap := wrap.New(link)
	if err := wrap.RegisterAnyParamFn(Name, "put", 3, i.putFnVariadic); err != nil {
		return err
	}
	return wrap.RegisterAnyParamFn(Name, "get", 2, i.getFnVariadic)
}

func (i *Import) putFnVariadic(caller *program.Caller, args ...int64) (*types.Val, error) {
	if len(args) != 3 {
		return nil, errors.New("expected 3 arguments")
	}
	return i.putFn(caller, args[0], args[1], args[2])
}

func (i *Import) getFnVariadic(caller *program.Caller, args ...int64) (*types.Val, error) {
	if len(args) != 2 {
		return nil, errors.New("expected 2 arguments")
	}
	return i.getFn(caller, args[0], args[1])
}

func (i *Import) putFn(caller *program.Caller, id int64, key int64, value int64) (*types.Val, error) {
	memory, err := caller.Memory()
	if err != nil {
		i.log.Error("failed to get memory from caller",
			zap.Error(err),
		)
		return nil, err
	}

	programIDBytes, err := program.SmartPtr(id).Bytes(memory)
	if err != nil {
		i.log.Error("failed to read program id from memory",
			zap.Error(err),
		)
		return nil, err
	}

	keyBytes, err := program.SmartPtr(key).Bytes(memory)
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return nil, err
	}

	valueBytes, err := program.SmartPtr(value).Bytes(memory)

	if err != nil {
		i.log.Error("failed to read value from memory",
			zap.Error(err),
		)
		return nil, err
	}

	k := storage.ProgramPrefixKey(programIDBytes, keyBytes)
	err = i.mu.Insert(context.Background(), k, valueBytes)
	if err != nil {
		i.log.Error("failed to insert into storage",
			zap.Error(err),
		)
		return nil, err
	}

	return types.ValI64(0), nil
}

func (i *Import) getFn(caller *program.Caller, id int64, key int64) (*types.Val, error) {
	memory, err := caller.Memory()
	if err != nil {
		i.log.Error("failed to get memory from caller",
			zap.Error(err),
		)
		return nil, err
	}

	programIDBytes, err := program.SmartPtr(id).Bytes(memory)
	if err != nil {
		i.log.Error("failed to read program id from memory",
			zap.Error(err),
		)
		return nil, err
	}

	keyBytes, err := program.SmartPtr(key).Bytes(memory)
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return nil, err
	}
	k := storage.ProgramPrefixKey(programIDBytes, keyBytes)
	val, err := i.mu.GetValue(context.Background(), k)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			// TODO: return a more descriptive error
			return types.ValI64(-1), nil
		}
		i.log.Error("failed to get value from storage",
			zap.Error(err),
		)
		return nil, err
	}

	if err != nil {
		i.log.Error("failed to convert program id to id",
			zap.Error(err),
		)
		return nil, err
	}

	ptr, err := program.WriteBytes(memory, val)
	if err != nil {
		{
			i.log.Error("failed to write to memory",
				zap.Error(err),
			)
		}
		return nil, err
	}
	argPtr, err := program.NewSmartPtr(uint32(ptr), len(val))
	if err != nil {
		i.log.Error("failed to convert ptr to argument",
			zap.Error(err),
		)
		return nil, err
	}

	return types.ValI64(int64(argPtr)), nil
}
