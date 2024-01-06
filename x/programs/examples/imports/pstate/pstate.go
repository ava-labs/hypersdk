// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pstate

import (
	"context"
	"errors"

	"github.com/bytecodealliance/wasmtime-go/v14"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
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
	if err := link.RegisterImportFn(Name, "put", i.putFn); err != nil {
		return err
	}
	return link.RegisterImportFn(Name, "get", i.getFn)
}

func (i *Import) putFn(caller *wasmtime.Caller, id int64, key int64, value int64) int32 {
	memory := runtime.NewMemory(runtime.NewExportClient(caller))
	// memory := runtime.NewMemory(client)
	programIDBytes, err := runtime.SmartPtr(id).Bytes(memory)
	if err != nil {
		i.log.Error("failed to read program id from memory",
			zap.Error(err),
		)
		return -1
	}

	keyBytes, err := runtime.SmartPtr(key).Bytes(memory)
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return -1
	}

	valueBytes, err := runtime.SmartPtr(value).Bytes(memory)

	if err != nil {
		i.log.Error("failed to read value from memory",
			zap.Error(err),
		)
		return -1
	}

	k := storage.ProgramPrefixKey(programIDBytes, keyBytes)
	err = i.mu.Insert(context.Background(), k, valueBytes)
	if err != nil {
		i.log.Error("failed to insert into storage",
			zap.Error(err),
		)
		return -1
	}

	return 0
}

func (i *Import) getFn(caller *wasmtime.Caller, id int64, key int64) int64 {
	client := runtime.NewExportClient(caller)
	memory := runtime.NewMemory(client)
	programIDBytes, err := runtime.SmartPtr(id).Bytes(memory)
	if err != nil {
		i.log.Error("failed to read program id from memory",
			zap.Error(err),
		)
		return -1
	}

	keyBytes, err := runtime.SmartPtr(key).Bytes(memory)
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return -1
	}
	k := storage.ProgramPrefixKey(programIDBytes, keyBytes)
	val, err := i.mu.GetValue(context.Background(), k)
	if err != nil {
		if !errors.Is(err, database.ErrNotFound) {
			i.log.Error("failed to get value from storage",
				zap.Error(err),
			)
		}
		return -1
	}
	if err != nil {
		i.log.Error("failed to convert program id to id",
			zap.Error(err),
		)
		return -1
	}

	ptr, err := runtime.WriteBytes(memory, val)
	if err != nil {
		{
			i.log.Error("failed to write to memory",
				zap.Error(err),
			)
		}
		return -1
	}
	argPtr, err := runtime.NewSmartPtr(uint32(ptr), len(val))
	if err != nil {
		i.log.Error("failed to convert ptr to argument",
			zap.Error(err),
		)
		return -1
	}

	return int64(argPtr)
}
