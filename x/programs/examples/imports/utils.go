// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package imports

import (
	"encoding/binary"

	"github.com/ava-labs/hypersdk/consts"
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

// PrependLength returns the length of [bytes] prepended to [bytes].
func PrependLength(bytes []byte) []byte {
	length := uint32(len(bytes))
	lenBytes := make([]byte, consts.Uint32Len)
	binary.BigEndian.PutUint32(lenBytes, length)
	return append(lenBytes, bytes...)
}
