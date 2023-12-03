// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package imports

import (
	"encoding/binary"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

// GetBytesFromPtr returns the bytes at [ptr] in [client] memory.
func GetBytesFromPtr(client runtime.WasmtimeExportClient, ptr int64) ([]byte, error) {
	memory := runtime.NewMemory(client)

	// first 4 bytes represent the length
	lenBytes, err := memory.Range(uint64(ptr), consts.Uint32Len)
	if err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBytes)

	// The following [length] bytes represent the bytes to return
	bytes, err := memory.Range(uint64(ptr+consts.Uint32Len), uint64(length))
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
