// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pstate

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/near/borsh-go"
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

func (i *Import) Register(link *host.Link, _ *program.Context) error {
	i.meter = link.Meter()
	wrap := wrap.New(link)
	if err := wrap.RegisterAnyParamFn(Name, "put", 2, i.putFnVariadic); err != nil {
		return err
	}
	if err := wrap.RegisterAnyParamFn(Name, "get", 2, i.getFnVariadic); err != nil {
		return err
	}
	if err := wrap.RegisterAnyParamFn(Name, "delete", 2, i.deleteFnVariadic); err != nil {
		return err
	}
	return wrap.RegisterAnyParamFn(Name, "log", 2, i.logFnVariadic)
}

func (i *Import) putFnVariadic(caller *program.Caller, args ...int32) (*types.Val, error) {
	if len(args) != 2 {
		return nil, errors.New("expected 2 arguments")
	}
	return i.putFn(caller, args[0], args[1])
}

func (i *Import) getFnVariadic(caller *program.Caller, args ...int32) (*types.Val, error) {
	if len(args) != 2 {
		return nil, errors.New("expected 2 arguments")
	}
	return i.getFn(caller, args[0], args[1])
}

func (i *Import) deleteFnVariadic(caller *program.Caller, args ...int32) (*types.Val, error) {
	if len(args) != 2 {
		return nil, errors.New("expected 2 arguments")
	}
	return i.deleteFn(caller, args[0], args[1])
}

func (i *Import) logFnVariadic(caller *program.Caller, args ...int32) (*types.Val, error) {
	if len(args) != 2 {
		return nil, errors.New("expected 2 arguments")
	}
	return i.logFn(caller, args[0], args[1])
}

type putArgs struct {
	ProgramID [32]byte
	Key       []byte
	Value     []byte
}

func (i *Import) putFn(caller *program.Caller, memOffset int32, size int32) (*types.Val, error) {
	memory, err := caller.Memory()
	if err != nil {
		i.log.Error("failed to get memory from caller",
			zap.Error(err),
		)
		return nil, err
	}

	bytes, err := memory.Range(uint32(memOffset), uint32(size))
	if err != nil {
		i.log.Error("failed to read args from program memory",
			zap.Error(err),
		)
		return nil, err
	}

	args := putArgs{}
	err = borsh.Deserialize(&args, bytes)
	if err != nil {
		i.log.Error("failed to deserialize args",
			zap.Error(err),
		)
		return nil, err
	}

	k := storage.ProgramPrefixKey(args.ProgramID[:], args.Key)
	err = i.mu.Insert(context.Background(), k, args.Value)
	if err != nil {
		i.log.Error("failed to insert into storage",
			zap.Error(err),
		)
		return nil, err
	}

	return types.ValI32(0), nil
}

type getAndDeleteArgs struct {
	ProgramID [32]byte
	Key       []byte
}

func (i *Import) getFn(caller *program.Caller, memOffset int32, size int32) (*types.Val, error) {
	memory, err := caller.Memory()
	if err != nil {
		i.log.Error("failed to get memory from caller",
			zap.Error(err),
		)
		return nil, err
	}

	bytes, err := memory.Range(uint32(memOffset), uint32(size))
	if err != nil {
		i.log.Error("failed to read args from program memory",
			zap.Error(err),
		)
		return nil, err
	}

	args := getAndDeleteArgs{}
	err = borsh.Deserialize(&args, bytes)
	if err != nil {
		i.log.Error("failed to deserialize args",
			zap.Error(err),
		)
		return nil, err
	}

	k := storage.ProgramPrefixKey(args.ProgramID[:], args.Key)
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

func (i *Import) deleteFn(caller *program.Caller, memOffset int32, size int32) (*types.Val, error) {
	memory, err := caller.Memory()
	if err != nil {
		i.log.Error("failed to get memory from caller",
			zap.Error(err),
		)
		return nil, err
	}

	bytes, err := memory.Range(uint32(memOffset), uint32(size))
	if err != nil {
		i.log.Error("failed to read args from program memory",
			zap.Error(err),
		)
		return nil, err
	}

	args := getAndDeleteArgs{}
	err = borsh.Deserialize(&args, bytes)
	if err != nil {
		i.log.Error("failed to deserialize args",
			zap.Error(err),
		)
		return nil, err
	}

	k := storage.ProgramPrefixKey(args.ProgramID[:], args.Key)
	bytes, err = i.mu.GetValue(context.Background(), k)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			// [0] represents `None`
			val, err := program.WriteBytes(memory, []byte{0})
			if err != nil {
				i.log.Error("failed to write to memory",
					zap.Error(err),
				)
				return nil, err
			}

			return types.ValI32(int32(val)), nil
		}

		i.log.Error("failed to get value from storage",
			zap.Error(err),
		)
		return nil, err
	}

	// 1 is the discriminant for `Some`
	bytes = append([]byte{1}, bytes...)

	ptr, err := program.WriteBytes(memory, bytes)
	if err != nil {
		{
			i.log.Error("failed to write to memory",
				zap.Error(err),
			)
		}
		return nil, err
	}

	if err := i.mu.Remove(context.Background(), k); err != nil {
		i.log.Error("failed to delete value from storage",
			zap.Error(err),
		)
		return nil, err
	}

	return types.ValI32(int32(ptr)), nil
}

func (i *Import) logFn(caller *program.Caller, memOffset int32, size int32) (*types.Val, error) {
	memory, err := caller.Memory()
	if err != nil {
		i.log.Error("failed to get memory from caller",
			zap.Error(err),
		)
		return nil, err
	}

	bytes, err := memory.Range(uint32(memOffset), uint32(size))
	if err != nil {
		i.log.Error("failed to read args from program memory",
			zap.Error(err),
		)
		return nil, err
	}

	fmt.Fprintf(os.Stderr, "%s\n", bytes)

	return types.ValI32(0), nil
}
