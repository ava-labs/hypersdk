// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pstate

import (
	"context"
	"errors"
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

const Name = "state"

var _ runtime.Import = &Import{}

// New returns a program storage module capable of storing arbitrary bytes
// in the program's namespace.
func New(log logging.Logger, mu state.Mutable) runtime.Import {
	return &Import{mu: mu, log: log}
}

type Import struct {
	mu         state.Mutable
	log        logging.Logger
	meter      runtime.Meter
	registered bool
}

func (i *Import) Name() string {
	return Name
}

func (i *Import) Register(link runtime.Link, meter runtime.Meter, _ runtime.SupportedImports) error {
	if i.registered {
		return fmt.Errorf("import module already registered: %q", Name)
	}
	i.meter = meter
	i.registered = true

	if err := link.FuncWrap(Name, "put", i.putFn); err != nil {
		return err
	}
	if err := link.FuncWrap(Name, "get", i.getFn); err != nil {
		return err
	}
	if err := link.FuncWrap(Name, "len", i.getLenFn); err != nil {
		return err
	}

	return nil
}

func (i *Import) putFn(caller *wasmtime.Caller, idPtr int64, keyPtr int32, keyLength int32, valuePtr int32, valueLength int32) int32 {
	memory := runtime.NewMemory(runtime.NewExportClient(caller))
	programIDBytes, err := memory.Range(uint64(idPtr), uint64(ids.IDLen))
	if err != nil {
		i.log.Error("failed to read program id from memory",
			zap.Error(err),
		)
		return -1
	}

	keyBytes, err := memory.Range(uint64(keyPtr), uint64(keyLength))
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return -1
	}

	valueBytes, err := memory.Range(uint64(valuePtr), uint64(valueLength))
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

func (i *Import) getLenFn(caller *wasmtime.Caller, idPtr int64, keyPtr int32, keyLength int32) int32 {
	memory := runtime.NewMemory(runtime.NewExportClient(caller))
	programIDBytes, err := memory.Range(uint64(idPtr), uint64(ids.IDLen))
	if err != nil {
		i.log.Error("failed to read program id from memory",
			zap.Error(err),
		)
		return -1
	}

	keyBytes, err := memory.Range(uint64(keyPtr), uint64(keyLength))
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

	return int32(len(val))
}

func (i *Import) getFn(caller *wasmtime.Caller, idPtr int64, keyPtr int32, keyLength int32, valLength int32) int32 {
	memory := runtime.NewMemory(runtime.NewExportClient(caller))
	programIDBytes, err := memory.Range(uint64(idPtr), uint64(ids.IDLen))
	if err != nil {
		i.log.Error("failed to read program id from memory",
			zap.Error(err),
		)
		return -1
	}

	keyBytes, err := memory.Range(uint64(keyPtr), uint64(keyLength))
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

	return int32(ptr)
}
