// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"fmt"
	"math"
	"runtime"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

// Memory is a wrapper around a wasmtime.Memory
type Memory struct {
	store   wasmtime.Storelike
	inner   *wasmtime.Memory
	allocFn *wasmtime.Func
}

// NewMemory creates a new memory wrapper.
func NewMemory(inner *wasmtime.Memory, allocFn *wasmtime.Func, store wasmtime.Storelike) *Memory {
	return &Memory{
		inner:   inner,
		store:   store,
		allocFn: allocFn,
	}
}

func (m *Memory) Range(offset uint32, length uint32) ([]byte, error) {
	err := m.ensureValidOffset(offset, length)
	if err != nil {
		return nil, err
	}
	mem := m.inner
	runtime.KeepAlive(mem)

	data := mem.UnsafeData(m.store)
	buf := make([]byte, length)

	// copy data from memory to buf to ensure it is not GCed.
	copy(buf, data[offset:offset+length])

	return buf, nil
}

func (m *Memory) Write(offset uint32, buf []byte) error {
	if len(buf) > math.MaxUint32 {
		return ErrOverflow
	}

	err := m.ensureValidOffset(offset, uint32(len(buf)))
	if err != nil {
		return err
	}

	mem := m.inner
	runtime.KeepAlive(mem)

	data := mem.UnsafeData(m.store)
	copy(data[offset:], buf)

	return nil
}

func (m *Memory) ensureValidOffset(offset uint32, length uint32) error {
	if offset == 0 && length == 0 {
		return nil
	}
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

func (m *Memory) Alloc(length uint32) (uint32, error) {
	err := m.ensureValidOffset(0, length)
	if err != nil {
		return 0, err
	}

	runtime.KeepAlive(m.inner)

	allocFn := m.allocFn
	if allocFn == nil {
		return 0, fmt.Errorf("%w: function not found: %s", ErrInvalidType, AllocFnName)
	}

	if !EnsureUint32ToInt32(length) {
		return 0, fmt.Errorf("%w: %d", ErrOverflow, length)
	}

	result, err := allocFn.Call(m.store, int32(length))
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

	return uint32(addr), nil
}

func (m *Memory) Grow(delta uint64) (uint32, error) {
	mem := m.inner
	runtime.KeepAlive(mem)
	length, err := mem.Grow(m.store, delta)
	if err != nil {
		return 0, err
	}
	if length > math.MaxUint32 {
		return 0, ErrOverflow
	}
	return uint32(length), nil
}

func (m *Memory) Len() (uint32, error) {
	mem := m.inner
	runtime.KeepAlive(mem)

	size := mem.DataSize(m.store)

	if uint64(size) > math.MaxUint32 {
		return 0, ErrOverflow
	}

	return uint32(size), nil
}

// WriteBytes is a helper function that allocates memory and writes the given
// bytes to the memory returning the offset.
func WriteBytes(m *Memory, buf []byte) (uint32, error) {
	if len(buf) > math.MaxUint32 {
		return 0, ErrOverflow
	}
	offset, err := m.Alloc(uint32(len(buf)))
	if err != nil {
		return 0, err
	}
	err = m.Write(offset, buf)
	if err != nil {
		return 0, err
	}

	return offset, nil
}

// SmartPtr is an int64 where the first 4 bytes represent the length of the bytes
// and the following 4 bytes represent a pointer to WASM memory where the bytes are stored.
type SmartPtr int64

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
func (s SmartPtr) Bytes(memory *Memory) ([]byte, error) {
	// read the range of PtrOffset + length from memory
	bytes, err := memory.Range(s.PtrOffset(), s.Len())
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// BytesToSmartPtr writes [bytes] to memory and returns the resulting SmartPtr.
func BytesToSmartPtr(bytes []byte, memory *Memory) (SmartPtr, error) {
	ptr, err := WriteBytes(memory, bytes)
	if err != nil {
		return 0, err
	}

	return NewSmartPtr(ptr, len(bytes))
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
