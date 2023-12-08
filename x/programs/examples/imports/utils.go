// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package imports

import (
	"fmt"

	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

// GetBytesFromPtr returns the bytes at [ptr] in [client] memory.
func GetBytesFromPtr(client runtime.WasmtimeExportClient, ptrArg int64) ([]byte, error) {
	memory := runtime.NewMemory(client)

	// first 4 bytes represent the length of the bytes to return
	length := ptrArg >> 32

	// grab the ptr which is the right most 4 bytes
	mask := ^uint32(0)
	ptr := ptrArg & int64(mask)


	// The following [length] bytes represent the bytes to return
	bytes, err := memory.Range(uint64(ptr), uint64(length))
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// // PrependLength returns the length of [bytes] prepended to [bytes].
// func PrependLength(bytes []byte) []byte {
// 	length := uint32(len(bytes))
// 	lenBytes := make([]byte, consts.Uint32Len)
// 	binary.BigEndian.PutUint32(lenBytes, length)
// 	return append(lenBytes, bytes...)
// }


// toPtrArgument converts [ptr] to an int64 with the length of [bytes] in the upper 32 bits
// and [ptr] in the lower 32 bits.
func ToPtrArgument(ptr int64, len uint32) int64 {

	// // ensure length of bytes is not greater than int32
	// if !runtime.EnsureInt64ToInt32(int64(len(bytes))) {
	// 	return 0, fmt.Errorf("length of bytes is greater than int32")
	// }
	// convert to int64 with length of bytes in the upper 32 bits and ptr in the lower 32 bits
	argPtr := int64(len) << 32
	argPtr |= int64((uint32(ptr)))
	fmt.Println("ptr and len", ptr, len)
	return argPtr
}
