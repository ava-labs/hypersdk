// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"fmt"
	"math"

	"github.com/near/borsh-go"
)

func EnsureInt64ToInt32(v int64) bool {
	return v >= math.MinInt32 && v <= math.MaxInt32
}

func EnsureUint32ToInt32(v uint32) bool {
	return v <= math.MaxInt32
}

// SmartPtr is an int64 where the first 4 bytes represent the length of the bytes
// and the following 4 bytes represent a pointer to WASM memeory where the bytes are stored.
type SmartPtr int64;

// ptrMask is used to get the lower 32 bits of a SmartPtr
var ptrMask = ^uint32(0)

// GetBytesFromArgPtr returns the bytes at [ptr] in [client] memory.
func FromSmartPtr(client WasmtimeExportClient, smartPtr SmartPtr) ([]byte, error) {
	memory := NewMemory(client)

	ptr, length := splitSmartPtr(smartPtr)

	// The following [length] bytes represent the bytes to return
	bytes, err := memory.Range(uint64(ptr), uint64(length))
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// ToSmartPtr converts [ptr] and [len] to a SmartPtr.
func ToSmartPtr(ptr uint32, len uint32) (SmartPtr, error) {
	// ensure length of bytes is not greater than int32 to prevent overflow
	if !EnsureUint32ToInt32(len) {
		return 0, fmt.Errorf("length of bytes is greater than int32")
	}

	// convert to int64 with length of bytes in the upper 32 bits and ptr in the lower 32 bits
	smartPtr := int64(len) << 32
	smartPtr |= int64(ptr)
	return SmartPtr(smartPtr), nil
}

// splitSmartPtr splits the upper 32 and lower 32 bits of [smartPtr] 
// into a length and ptr respectively.
func splitSmartPtr(smartPtr SmartPtr) (uint32, uint32) {	
	length := smartPtr >> 32
	ptr := int64(smartPtr) & int64(ptrMask)

	return uint32(ptr), uint32(length)
}	

// NewRuntimePtr allocates memory in the runtime and writes [bytes] to it.
// It returns the pointer of the allocated memory.
func NewRuntimePtr(ctx context.Context, bytes []byte, rt Runtime) (int64, error) {
	amountToAllocate := len(bytes)

	ptr, err := rt.Memory().Alloc(uint64(amountToAllocate))
	if err != nil {
		return 0, err
	}

	// write programID to memory which we will later pass to the program
	err = rt.Memory().Write(ptr, bytes)
	if err != nil {
		return 0, err
	}

	return int64(ptr), err
}

// SerializeParameter serializes [obj] using Borsh 
func SerializeParameter(obj interface{}) ([]byte, error) {
	bytes, err := borsh.Serialize(obj)
	return bytes, err
}

// NewSmartPtr creates a SmartPtr by serializing the provided object using Borsh,
// allocating memory for it in [rt], and constructing a SmartPtr
// from the resulting pointer and length of the serialized bytes. 
func NewSmartPtr(ctx context.Context, obj interface{}, rt Runtime) (SmartPtr, error) {
	bytes, err := SerializeParameter(obj)
	if err != nil {
		return 0, err
	}
	ptr, err := NewRuntimePtr(ctx, bytes, rt)
	if err != nil {
		return 0, err
	}

	return ToSmartPtr(uint32(ptr), uint32(len(bytes)))
}