// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"encoding/binary"
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

func (m *Memory) RangeFromInt64(param int64) ([]byte, error) {
	return m.Range(splitInt64(param))
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

// CallParam defines a value to be passed to a guest function.
type CallParam struct {
	Value interface{} `json,yaml:"value"`
}

// WriteParams is a helper function that writes the given params to memory if non integer.
// Supported types include int, uint64 and string.
func WriteParams(m *Memory, p []CallParam) ([]uint64, error) {
	params := []uint64{}
	for _, param := range p {
		switch v := param.Value.(type) {
		case string:
			ptr, err := WriteBytes(m, []byte(v))
			if err != nil {
				return nil, err
			}
			params = append(params, uint64(ptr))
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

// Int64ToBytes converts an int64 to a byte array. The assumption is that the i64 represents
func Int64ToBytes(m *Memory, value int64) ([]byte, error) {
	if value < 0 {
		return nil, ErrUnderflow
	}

	return m.Range(splitInt64(value))
}

func splitInt64(param int64) (uint32, uint32) {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uint64(param))

	// first 4 bytes are length
	length := binary.BigEndian.Uint32(bytes[:4])
	// last 4 bytes are the pointer offset
	offset := binary.BigEndian.Uint32(bytes[4:])

	return offset, length
}

func BytesToInt64(m *Memory, bytes []byte) (int64, error) {
	if len(bytes) > math.MaxUint32 {
		return 0, ErrOverflow
	}
	offset, err := m.Alloc(uint32(len(bytes)))
	if err != nil {
		return 0, err
	}
	err = m.Write(offset, bytes)
	if err != nil {
		return 0, err
	}

	return joinInt64(offset, uint32(len(bytes))), nil
}

func joinInt64(offset uint32, length uint32) int64 {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint32(bytes[:4], length)
	binary.BigEndian.PutUint32(bytes[4:], offset)

	return int64(binary.BigEndian.Uint64(bytes))
}
