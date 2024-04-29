// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pstate

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

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

func (*Import) Name() string {
	return Name
}

func (i *Import) Register(link *host.Link, _ program.Context) error {
	i.meter = link.Meter()
	wrap := wrap.New(link)
	if err := wrap.RegisterAnyParamFn(Name, "put", 6, i.putFnVariadic); err != nil {
		return err
	}
	if err := wrap.RegisterAnyParamFn(Name, "get", 4, i.getFnVariadic); err != nil {
		return err
	}

	return wrap.RegisterAnyParamFn(Name, "delete", 4, i.deleteFnVariadic)
}

func (i *Import) putFnVariadic(caller *program.Caller, args ...int32) (*types.Val, error) {
	if len(args) != 6 {
		return nil, errors.New("expected 6 arguments")
	}
	return i.putFn(caller, args[0], args[1], args[2], args[3], args[4], args[5])
}

func (i *Import) getFnVariadic(caller *program.Caller, args ...int32) (*types.Val, error) {
	if len(args) != 4 {
		return nil, errors.New("expected 4 arguments")
	}
	return i.getFn(caller, args[0], args[1], args[2], args[3])
}

func (i *Import) deleteFnVariadic(caller *program.Caller, args ...int32) (*types.Val, error) {
	if len(args) != 4 {
		return nil, errors.New("expected 4 arguments")
	}
	return i.deleteFn(caller, args[0], args[1], args[2], args[3])
}

func (i *Import) putFn(caller *program.Caller, idPtr int32, idLen int32, keyPtr int32, keyLen int32, valuePtr int32, valueLen int32) (*types.Val, error) {
	memory, err := caller.Memory()
	if err != nil {
		i.log.Error("failed to get memory from caller",
			zap.Error(err),
		)
		return nil, err
	}

	programIDBytes, err := memory.Range(uint32(idPtr), uint32(idLen))
	if err != nil {
		i.log.Error("failed to read program id from memory",
			zap.Error(err),
		)
		return nil, err
	}

	keyBytes, err := memory.Range(uint32(keyPtr), uint32(keyLen))
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return nil, err
	}

	valueBytes, err := memory.Range(uint32(valuePtr), uint32(valueLen))
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

	return types.ValI32(0), nil
}

func (i *Import) getFn(caller *program.Caller, idPtr int32, idLen int32, keyPtr int32, keyLen int32) (*types.Val, error) {
	memory, err := caller.Memory()
	if err != nil {
		i.log.Error("failed to get memory from caller",
			zap.Error(err),
		)
		return nil, err
	}

	programIDBytes, err := memory.Range(uint32(idPtr), uint32(idLen))
	if err != nil {
		i.log.Error("failed to read program id from memory",
			zap.Error(err),
		)
		return nil, err
	}

	keyBytes, err := memory.Range(uint32(keyPtr), uint32(keyLen))
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
			return types.ValI32(-1), nil
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

	valPtr, err := program.WriteBytes(memory, val)
	if err != nil {
		{
			i.log.Error("failed to write to memory",
				zap.Error(err),
			)
		}
		return nil, err
	}
	_, err = memory.Range(valPtr, uint32(len(val)))
	if err != nil {
		i.log.Error("failed to convert ptr to argument",
			zap.Error(err),
		)
		return nil, err
	}

	return types.ValI32(int32(valPtr)), nil
}

func (i *Import) deleteFn(caller *program.Caller, idPtr int32, idLen int32, keyPtr int32, keyLen int32) (*types.Val, error) {
	memory, err := caller.Memory()
	if err != nil {
		i.log.Error("failed to get memory from caller",
			zap.Error(err),
		)
		return nil, err
	}

	programIDBytes, err := memory.Range(uint32(idPtr), uint32(idLen))
	if err != nil {
		i.log.Error("failed to read program id from memory",
			zap.Error(err),
		)
		return nil, err
	}

	keyBytes, err := memory.Range(uint32(keyPtr), uint32(keyLen))
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return nil, err
	}

	k := storage.ProgramPrefixKey(programIDBytes, keyBytes)
	if err := i.mu.Remove(context.Background(), k); err != nil {
		i.log.Error("failed to remove from storage", zap.Error(err))
		return types.ValI32(-1), nil
	}
	return types.ValI32(0), nil
}
