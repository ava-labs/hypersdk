// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package hashmap

import (
	"github.com/bytecodealliance/wasmtime-go/v12"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

// GlobalStorage is a global variable that holds program state.
var GlobalStorage = storage{
	state: make(map[int64]maps),
}

type maps map[string][]byte

// Key value store for program data
type storage struct {
	state map[int64]maps
}

const Name = "map"

func New(log logging.Logger, mu state.Mutable) *Import {
	return &Import{mu: mu, log: log}
}

func AddProgramID(id int64) {
	GlobalStorage.state[id] = make(maps)
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
	if err := link.FuncWrap(Name, "store_bytes", i.storeBytesFn); err != nil {
		return err
	}
	if err := link.FuncWrap(Name, "get_bytes_len", i.getBytesLenFn); err != nil {
		return err
	}
	if err := link.FuncWrap(Name, "get_bytes", i.getBytesFn); err != nil {
		return err
	}

	i.meter = meter

	return nil
}

func (i *Import) storeBytesFn(caller *wasmtime.Caller, id int64, keyPtr int32, keyLength int32, valuePtr int32, valueLength int32) int32 {
	memory := runtime.NewMemory(runtime.NewExportClient(caller))

	_, ok := GlobalStorage.state[int64(id)]
	if !ok {
		i.log.Error("failed to find program id in storage")
		return -1
	}

	keyBytes, err := memory.Range(uint32(keyPtr), uint32(keyLength))
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return -1
	}

	valueBytes, err := memory.Range(uint32(valuePtr), uint32(valueLength))
	if err != nil {
		i.log.Error("failed to read value from memory",
			zap.Error(err),
		)
		return -1
	}

	// Need to copy the value because the GC can collect the value after this function returns
	// is this still true?
	copiedValue := make([]byte, len(valueBytes))
	copy(copiedValue, valueBytes)
	GlobalStorage.state[id][string(keyBytes)] = copiedValue

	return 0
}

func (i *Import) getBytesLenFn(caller *wasmtime.Caller, id int64, keyPtr int32, keyLength int32) int32 {
	memory := runtime.NewMemory(runtime.NewExportClient(caller))

	_, ok := GlobalStorage.state[int64(id)]
	if !ok {
		i.log.Error("failed to find program id in storage")
		return -1
	}

	keyBytes, err := memory.Range(uint32(keyPtr), uint32(keyLength))
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return -1
	}
	val, ok := GlobalStorage.state[id][string(keyBytes)]
	if !ok {
		return -1
	}

	return int32(len(val))
}

func (i *Import) getBytesFn(caller *wasmtime.Caller, id int64, keyPtr int32, keyLength int32, valLength int32) int32 {
	memory := runtime.NewMemory(runtime.NewExportClient(caller))

	_, ok := GlobalStorage.state[int64(id)]
	if !ok {
		i.log.Error("failed to find program id in storage")
		return -1
	}

	keyBytes, err := memory.Range(uint32(keyPtr), uint32(keyLength))
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return -1
	}
	val, ok := GlobalStorage.state[id][string(keyBytes)]
	if !ok {
		return -1
	}

	ptr, err := runtime.WriteBytes(memory, val)
	if err != nil {
		i.log.Error("failed to write to memory",
			zap.Error(err),
		)
		return -1
	}

	return int32(ptr)
}
