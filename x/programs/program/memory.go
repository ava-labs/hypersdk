// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"fmt"
	"math"
	"runtime"
)

var _ Memory = (*memory)(nil)

type memory struct {
	client WasmtimeExportClient
}

// SmartPtr is an int64 where the first 4 bytes represent the length of the bytes
// and the following 4 bytes represent a pointer to WASM memeory where the bytes are stored.
type SmartPtr int64

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

	if length > math.MaxInt32 {
		return 0, fmt.Errorf("failed to allocate memory: %w", ErrIntegerConversionOverflow)
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
func WriteBytes(m Memory, buf []byte) (int64, error) {
	offset, err := m.Alloc(uint64(len(buf)))
	if err != nil {
		return 0, err
	}
	err = m.Write(offset, buf)
	if err != nil {
		return 0, err
	}

	return int64(offset), nil
}

// CallParam defines a value to be passed to a guest function.
type CallParam struct {
	Value interface{} `json,yaml:"value"`
}

// WriteParams is a helper function that writes the given params to memory if non integer.
// Supported types include int, uint64 and string.
func WriteParams(m Memory, p []CallParam) ([]int64, error) {
	params := []int64{}
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
			params = append(params, int64(v))
		case uint64:
			if v > math.MaxInt64 {
				return nil, fmt.Errorf("failed to write param: %w", ErrIntegerConversionOverflow)
			}
			params = append(params, int64(v))
		default:
			return nil, fmt.Errorf("%w: support types int, uint64 and string", ErrInvalidParamType)
		}
	}

	return params, nil
}

// Get returns the int64 value of [s].
func (s SmartPtr) Get() int64 {
	return int64(s)
}

// Len returns the length of the bytes stored in memory by [s].
func (s SmartPtr) Len() uint32 {
	return uint32(s >> 32)
}

// PtrOffset returns the offset of the bytes stored in memory by [s].
func (s SmartPtr) PtrOffset() uint32 {
	return uint32(s)
}

// Bytes returns the bytes stored in memory by [s].
func (s SmartPtr) Bytes(memory Memory) ([]byte, error) {
	// read the range of PtrOffset + length from memory
	bytes, err := memory.Range(uint64(s.PtrOffset()), uint64(s.Len()))
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// BytesToSmartPtr writes [bytes] to memory and returns the resulting SmartPtr.
func BytesToSmartPtr(bytes []byte, memory Memory) (SmartPtr, error) {
	ptr, err := WriteBytes(memory, bytes)
	if err != nil {
		return 0, err
	}

	return NewSmartPtr(uint32(ptr), len(bytes))
}

// NewSmartPtr returns a SmartPtr from [ptr] and [len].
func NewSmartPtr(ptr uint32, len int) (SmartPtr, error) {
	// ensure length of bytes is not greater than int32 to prevent overflow
	if !EnsureIntToInt32(len) {
		return 0, fmt.Errorf("length of bytes is greater than int32")
	}

	lenUpperBits := int64(len) << 32
	ptrLowerBits := int64(ptr)

	return SmartPtr(lenUpperBits | ptrLowerBits), nil
}
