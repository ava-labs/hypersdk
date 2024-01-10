// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package memory

import (
	"fmt"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"math"
)

const (
	MemoryPageSize = 64 * units.KiB
)

// WasmTimeMemory is a wrapper around a wasmtime.Memory
type WasmTimeMemory struct {
	store   wasmtime.Storelike
	inner   *wasmtime.Memory
	allocFn *wasmtime.Func
}

// NewWasmTimeMemory creates a new memory wrapper.
func NewWasmTimeMemory(inner *wasmtime.Memory, allocFn func(int32) int32, store wasmtime.Storelike) Memory {
	return &WasmTimeMemory{
		inner:   inner,
		store:   store,
		allocFn: allocFn,
	}
}

func (m *WasmTimeMemory) Load(slice Slice) ([]byte, error) {
	err := m.ensureValidSlice(slice)
	if err != nil {
		return nil, err
	}

	if uint32(slice.Ptr())+slice.Len() > uint32(m.inner.DataSize(m.store)) {
		return nil, ErrOutOfBounds
	}

	data := m.inner.UnsafeData(m.store)
	buf := make([]byte, slice.Len())

	// copy data from memory to buf to ensure it is not GCed.
	copy(buf, data[slice.Ptr():uint32(slice.Ptr())+slice.Len()])

	return buf, nil
}

func (m *WasmTimeMemory) Write(ptr Ptr, buf []byte) error {
	if len(buf) > math.MaxUint32 {
		return ErrOverflow
	}

	err := m.ensureValidOffset(uint32(ptr), uint32(len(buf)))
	if err != nil {
		return err
	}

	data := m.inner.UnsafeData(m.store)
	copy(data[ptr:], buf)

	return nil
}

func (m *WasmTimeMemory) ensureValidOffset(offset uint32, len uint32) error {
	if offset == 0 && len == 0 {
		return nil
	}
	memCap, err := m.Capacity()
	if err != nil {
		return err
	}

	// verify available memory is large enough
	if offset+len > memCap {
		return ErrOverflow
	}

	return nil
}

func (m *WasmTimeMemory) ensureValidSlice(s Slice) error {
	return m.ensureValidOffset(uint32(s.Ptr()), s.Len())
}

func (m *WasmTimeMemory) alloc(len uint32) (Ptr, error) {
	err := m.ensureValidOffset(0, len)
	if err != nil {
		return 0, err
	}

	allocFn := m.allocFn
	if allocFn == nil {
		return 0, fmt.Errorf("%w: function not found: alloc", ErrInvalidType)
	}

	if len >= math.MaxInt32 {
		return 0, fmt.Errorf("%w: %d", ErrOverflow, len)
	}

	result, err := allocFn.Call(m.store, int32(len))
	if err != nil {
		return 0, err
	}

	addr, ok := result.(int32)
	if !ok {
		return 0, fmt.Errorf("%w: invalid result type: %T", ErrInvalidType, result)
	}
	if addr < 0 {
		return 0, ErrUnderflow
	}

	return Ptr(addr), nil
}

func (m *WasmTimeMemory) Grow(delta uint32) (uint32, error) {
	length, err := m.inner.Grow(m.store, uint64(delta))
	if err != nil {
		return 0, err
	}
	if length > math.MaxUint32 {
		return 0, ErrOverflow
	}
	return uint32(length), nil
}

func (m *WasmTimeMemory) Capacity() (uint32, error) {
	size := m.inner.DataSize(m.store)

	if uint64(size) > math.MaxUint32 {
		return 0, ErrOverflow
	}

	return uint32(size), nil
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
			slice, err := WriteBytes(m, []byte(v))
			if err != nil {
				return nil, err
			}
			params = append(params, uint64(Int64EncodeSlice(slice)))
		case int:
			if v < 0 {
				return nil, fmt.Errorf("failed to write param: %w", ErrUnderflow)
			}
			params = append(params, uint64(v))
		case uint64:
			params = append(params, v)
		default:
			return nil, fmt.Errorf("invalid param: supported types int, uint64 and string")
		}
	}

	return params, nil
}

// Int64Slice is an int64 where the first 4 bytes represent the length of the bytes
// and the following 4 bytes represent a pointer to WASM memory where the bytes are stored.
type Int64Slice int64

// Get returns the int64 value of [s].
func (s Int64Slice) Get() int64 {
	return int64(s)
}

// Len returns the length of the bytes stored in memory by [s].
func (s Int64Slice) Len() uint32 {
	return uint32(s >> 32)
}

// Ptr returns the ptr of the bytes stored in memory by [s].
func (s Int64Slice) Ptr() Ptr {
	return Ptr(s)
}

// Bytes returns the bytes stored in memory by [s].
func (s Int64Slice) Bytes(memory Memory) ([]byte, error) {
	return memory.Load(s)
}

// BytesToInt64Slice writes [bytes] to memory and returns the resulting SmartPtr.
func BytesToInt64Slice(bytes []byte, memory Memory) (Int64Slice, error) {
	ptr, err := WriteBytes(memory, bytes)
	if err != nil {
		return 0, err
	}

	return Int64EncodeSlice(ptr), nil
}

// NewInt64Slice returns a SmartPtr from [ptr] and [len].
func NewInt64Slice(ptr Ptr, len uint32) Int64Slice {
	lenUpperBits := int64(len) << 32
	ptrLowerBits := int64(ptr)

	return Int64Slice(lenUpperBits | ptrLowerBits)
}

func Int64EncodeSlice(slice Slice) Int64Slice {
	return NewInt64Slice(slice.Ptr(), slice.Len())
}
