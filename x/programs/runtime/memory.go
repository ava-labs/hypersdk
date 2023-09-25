// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"fmt"
	"runtime"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/bytecodealliance/wasmtime-go/v12"
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
	if uint64(offset)+uint64(length) > size {
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
		return 0, err
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

// IDFromMemory reads memory from the given offset and returns an ID.
func IDFromOffset(caller *wasmtime.Caller, offset int64) (ids.ID, error) {
	memory := NewMemory(NewExportClient(caller))
	assetBytes, err := memory.Range(uint64(offset), ids.IDLen)
	if err != nil {
		return ids.Empty, err
	}

	return ids.ToID(assetBytes)
}

// PublicKeyFromOffset reads memory from the given offset and returns an ed25519.PublicKey.
func PublicKeyFromOffset(caller *wasmtime.Caller, hrp string, offset int32) (ed25519.PublicKey, error) {
	memory := NewMemory(NewExportClient(caller))
	keyBytes, err := memory.Range(uint64(offset), ed25519.PublicKeyLen)
	if err != nil {
		return ed25519.EmptyPublicKey, err
	}

	return ed25519.ParseAddress(hrp, string(keyBytes))
}
