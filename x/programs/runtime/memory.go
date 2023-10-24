// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"fmt"
	"runtime"
)

var _ Memory = (*memory)(nil)

type memory struct {
	client WasmtimeExportClient
}

func NewMemory(client WasmtimeExportClient) *memory {
	return &memory{
		client: client,
	}
}

func (m *memory) Range(offset uint64, length uint64) ([]byte, error) {
	// memory is in local scope so we do not make assumptions on the
	// lifetime of *wasmtime.Memory. The garbage collector may collect
	// the memory if it is not referenced which could result in bugs.
	mem, err := m.client.GetMemory()
	if err != nil {
		return nil, err
	}
	size, err := m.Len()
	if err != nil {
		return nil, err
	}

	// verify available memory is large enough
	if offset+length > size {
		return nil, fmt.Errorf("read memory failed: %w", ErrInvalidMemorySize)
	}

	// ensure memory is not GCed during the life of this method
	runtime.KeepAlive(mem)

	data := mem.UnsafeData(m.client.Store())
	buf := make([]byte, length)

	// copy data from memory to buf to ensure it is not GCed.
	copy(buf, data[offset:offset+length])

	return buf, nil
}

func (m *memory) Write(offset uint64, buf []byte) error {
	mem, err := m.client.GetMemory()
	if err != nil {
		return err
	}

	max, err := m.Len()
	if err != nil {
		return err
	}

	lenBuf := len(buf)

	if max < offset+uint64(lenBuf) {
		return fmt.Errorf("write memory failed: %w: max: %d", ErrInvalidMemorySize, max)
	}

	data := mem.UnsafeData(m.client.Store())
	copy(data[offset:], buf)

	return nil
}

func (m *memory) Alloc(length uint64) (uint64, error) {
	fn, err := m.client.ExportedFunction(AllocFnName)
	if err != nil {
		return 0, err
	}
	result, err := fn.Call(m.client.Store(), int32(length))
	if err != nil {
		return 0, handleTrapError(err)
	}

	addr := result.(int32)
	if addr < 0 {
		return 0, ErrInvalidMemoryAddress
	}

	return uint64(addr), nil
}

func (m *memory) Grow(delta uint64) (uint64, error) {
	mem, err := m.client.GetMemory()
	if err != nil {
		return 0, err
	}

	return mem.Grow(m.client.Store(), delta)
}

func (m *memory) Len() (uint64, error) {
	mem, err := m.client.GetMemory()
	if err != nil {
		return 0, err
	}

	return uint64(mem.DataSize(m.client.Store())), nil
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
				return nil, fmt.Errorf("failed to write param: %w", ErrNegativeValue)
			}
			params = append(params, uint64(v))
		case uint64:
			params = append(params, v)
		default:
			return nil, fmt.Errorf("%w: support types int, uint64 and string", ErrInvalidParamType)
		}
	}

	return params, nil
}
