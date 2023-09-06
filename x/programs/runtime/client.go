// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/utils/logging"
)

const (
	allocFnName   = "alloc"
	deallocFnName = "dealloc"
)

// newClient returns a new runtime client capable of making calls to exported
// functions of the guest. All clients must be closed when no longer in use.
func newClient(log logging.Logger, mod api.Module, exported map[string]api.Function) *client {
	return &client{
		log:      log,
		mod:      mod,
		exported: exported,
		allocs:   make([]*alloc, 0),
	}
}

type client struct {
	// allocations managed by this client
	allocs   []*alloc
	exported map[string]api.Function
	mod      api.Module
	log      logging.Logger
}

type alloc struct {
	offset uint64
	len    uint64
}

func (c *client) call(ctx context.Context, name string, params ...uint64) ([]uint64, error) {
	api, ok := c.exported[name]
	if !ok {
		return nil, fmt.Errorf("failed to find exported function: %s", name)
	}
	return api.Call(ctx, params...)
}

// TODO: ensure properly metered
func (c *client) alloc(ctx context.Context, length uint64) (uint64, error) {
	result, err := c.call(ctx, allocFnName, length)
	if err != nil {
		return 0, err
	}
	return result[0], nil
}

func (c *client) dealloc(ctx context.Context, offset uint64, length uint64) error {
	_, err := c.call(ctx, deallocFnName, offset, length)
	return err
}

// TODO: ensure properly metered
func (c *client) readMemory(offset uint32, length uint32) ([]byte, bool) {
	return c.mod.Memory().Read(offset, length)
}

// TODO: ensure properly metered
func (c *client) writeMemory(offset uint32, buf []byte) error {
	ok := c.mod.Memory().Write(offset, buf)
	if !ok {
		return fmt.Errorf("failed to write at offset: %d size: %d", offset, c.mod.Memory().Size())
	}
	return nil
}

func (c *client) Close(ctx context.Context) {
	// deallocate all allocations made by this client
	for _, alloc := range c.allocs {
		err := c.dealloc(ctx, alloc.offset, alloc.len)
		c.log.Error("failed to deallocate memory",
			zap.Error(err),
		)
	}
	if err := c.mod.Close(ctx); err != nil {
		c.log.Error("failed to close wasm module",
			zap.Error(err),
		)
	}
}
