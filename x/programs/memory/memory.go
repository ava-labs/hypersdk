// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package memory

import (
	"fmt"
	"runtime"

	rt "github.com/ava-labs/hypersdk/x/programs/runtime"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

var (
	ErrOverflow    = fmt.Errorf("memory pointer overflow")
	ErrUnderflow   = fmt.Errorf("memory pointer underflow")
	ErrInvalidType = fmt.Errorf("invalid type")
)

const (
	AllocFnName   = "alloc"
	DeallocFnName = "dealloc"
	MemoryFnName  = "memory"
)

type Memory struct {
	inner  *wasmtime.Memory
	caller *wasmtime.Caller
}

func New(inner *wasmtime.Memory, caller *wasmtime.Caller) *Memory {
	return &Memory{
		inner:  inner,
		caller: caller,
	}
}

func (m *Memory) Range(offset uint64, length uint64) ([]byte, error) {
	err := m.ensureValidOffset(offset, length)
	if err != nil {
		return nil, err
	}

	// ensure memory is not GCed during the life of this method
	mem := m.inner
	runtime.KeepAlive(mem)

	data := mem.UnsafeData(m.caller)
	buf := make([]byte, length)

	// copy data from memory to buf to ensure it is not GCed.
	copy(buf, data[offset:offset+length])

	return buf, nil
}

func (m *Memory) Write(offset uint64, buf []byte) error {
	err := m.ensureValidOffset(offset, uint64(len(buf)))
	if err != nil {
		return err
	}

	data := m.inner.UnsafeData(m.caller)
	copy(data[offset:], buf)

	return nil
}

func (m *Memory) ensureValidOffset(offset uint64, length uint64) error {
	memLen, err := m.Len()
	if err != nil {
		return err
	}

	// verify available memory is large enough
	if offset+length > memLen {
		return ErrOverflow
	}

	return nil
}

func (m *Memory) Alloc(length uint64) (uint64, error) {
	err := m.ensureValidOffset(0, length)
	if err != nil {
		return 0, err
	}

	ext := m.caller.GetExport(AllocFnName)
	if ext == nil {
		return 0, fmt.Errorf("export not found: %s", AllocFnName)
	}

	fn := ext.Func()
	if fn == nil {
		return 0, fmt.Errorf("%w: function not found: %s", ErrInvalidType, AllocFnName)
	}

	result, err := fn.Call(m.caller, int32(length))
	if err != nil {
		return 0, rt.HandleTrapError(err)
	}

	addr, ok := result.(int32)
	if !ok {
		return 0, fmt.Errorf("%w: invalid result type: %T", ErrInvalidType, result)
	}
	if addr < 0 {
		return 0, ErrUnderflow
	}

	return uint64(addr), nil
}

func (m *Memory) Grow(delta uint64) (uint64, error) {
	mem := m.inner
	runtime.KeepAlive(mem)

	return mem.Grow(m.caller, delta)
}

func (m *Memory) Len() (uint64, error) {
	mem := m.inner
	runtime.KeepAlive(mem)

	return uint64(mem.DataSize(m.caller)), nil
}

// WriteBytes is a helper function that allocates memory and writes the given
// bytes to the memory returning the offset.
func WriteBytes(m Memory, buf []byte) (uint64, error) {
	offset, err := m.Alloc(uint64(len(buf)))
	if err != nil {
		return 0, err
	}
	err = m.Write(offset, buf)
	if err != nil {
		return 0, err
	}

	return offset, nil
}

// CallParam defines a value to be passed to a guest function.
type CallParam struct {
	Value interface{} `json,yaml:"value"`
}

// WriteParams is a helper function that writes the given params to memory if non integer.
// Supported types include int, uint64 and string.
func WriteParams(m Memory, p []CallParam) ([]uint64, error) {
	params := []uint64{}
	for _, param := range p {
		switch v := param.Value.(type) {
		case string:
			ptr, err := WriteBytes(m, []byte(v))
			if err != nil {
				return nil, err
			}
			params = append(params, ptr)
		case int:
			if v < 0 {
				return nil, fmt.Errorf("failed to write param: %w", ErrUnderflow)
			}
			params = append(params, uint64(v))
		case uint64:
			params = append(params, v)
		default:
			return nil, fmt.Errorf("%w: support types int, uint64 and string", runtime.ErrInvalidParamType)
		}
	}

	return params, nil
}
