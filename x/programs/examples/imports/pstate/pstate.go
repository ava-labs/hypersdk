// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pstate

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/program"
)

const Name = "state"

var _ host.Import = &Import{}

// New returns a program storage module capable of storing arbitrary bytes
// in the program's namespace.
func New(log logging.Logger, mu state.Mutable) host.Import {
	return &Import{mu: mu, log: log}
}

type Import struct {
	mu         state.Mutable
	log        logging.Logger
	registered bool
}

func (i *Import) Name() string {
	return Name
}

func (i *Import) Register(link *host.Link) error {
	if i.registered {
		return fmt.Errorf("import module already registered: %q", Name)
	}
	i.registered = true

	if err := link.RegisterFn(host.NewThreeParamImport(Name, "put", i.putFn)); err != nil {
		return err
	}
	if err := link.RegisterFn(host.NewThreeParamImport(Name, "get", i.getFn)); err != nil {
		return err
	}

	return nil
}

func (i *Import) putFn(
	caller *program.Caller,
	id,
	key,
	value int64,
) (*program.Val, error) {
	memory, err := caller.Memory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory: %w", err)
	}

	programIDBytes, err := program.Int64ToBytes(memory, id)
	if err != nil {
		return nil, err
	}

	keyBytes, err := program.Int64ToBytes(memory, key)
	if err != nil {
		return nil, fmt.Errorf("failed to read key from memory: %w", err)
	}

	valueBytes, err := program.Int64ToBytes(memory, value)
	if err != nil {
		return nil, fmt.Errorf("failed to read value from memory: %w", err)
	}

	k := storage.ProgramPrefixKey(programIDBytes, keyBytes)
	err = i.mu.Insert(context.Background(), k, valueBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to insert into storage: %w", err)
	}

	return nil, nil
}

func (i *Import) getFn(
	caller *program.Caller,
	id,
	key,
	value int64,
) (*program.Val, error) {
	memory, err := caller.Memory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory: %w", err)
	}

	programIDBytes, err := program.Int64ToBytes(memory, id)
	if err != nil {
		return nil, fmt.Errorf("failed to read program id from memory: %w", err)
	}

	keyBytes, err := program.Int64ToBytes(memory, key)
	if err != nil {
		return nil, fmt.Errorf("failed to read key from memory: %w", err)
	}

	k := storage.ProgramPrefixKey(programIDBytes, keyBytes)
	val, err := i.mu.GetValue(context.Background(), k)
	if err != nil {
		return nil, fmt.Errorf("failed to get value from storage: %w", err)
	}

	resp, err := program.BytesToInt64(memory, val)
	if err != nil {
		return nil, fmt.Errorf("failed to write value to memory: %w", err)
	}

	return program.ValI64(resp), nil
}
