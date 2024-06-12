package runtime

import (
	"context"

	"github.com/ava-labs/hypersdk/codec"
)

type CallContext struct {
	r            *WasmRuntime
	state        StateLoader
	actor        *codec.Address
	functionName *string
	program      *codec.Address
	params       []byte
	fuel         *uint64
}

func (c CallContext) CallProgram(ctx context.Context, info *CallInfo) ([]byte, error) {
	if c.state != nil {
		info.State = c.state
	}
	if c.actor != nil {
		info.Actor = *c.actor
	}
	if c.functionName != nil {
		info.FunctionName = *c.functionName
	}
	if c.program != nil {
		info.Program = *c.program
	}
	if c.params != nil {
		info.Params = c.params
	}
	if c.fuel != nil {
		info.Fuel = *c.fuel
	}
	return c.r.CallProgram(ctx, info)
}

func (c CallContext) WithStateLoader(loader StateLoader) CallContext {
	c.state = loader
	return c
}

func (c CallContext) WithActor(address codec.Address) CallContext {
	c.actor = &address
	return c
}

func (c CallContext) WithFunction(s string) CallContext {
	c.functionName = &s
	return c
}

func (c CallContext) WithProgram(address codec.Address) CallContext {
	c.program = &address
	return c
}

func (c CallContext) WithFuel(u uint64) CallContext {
	c.fuel = &u
	return c
}

func (c CallContext) WithParams(bytes []byte) CallContext {
	c.params = bytes
	return c
}
