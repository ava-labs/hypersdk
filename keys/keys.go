// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package keys

import (
	"encoding/binary"

	"github.com/ava-labs/hypersdk/consts"
)

const chunkSize = 64 // bytes

func Valid(key string) bool {
	return len(key) >= consts.Uint16Len
}

func MaxChunks(key string) (uint16, bool) {
	bkey := []byte(key)
	l := len(bkey)
	if l < consts.Uint16Len {
		return 0, false
	}
	return binary.BigEndian.Uint16(bkey[l-consts.Uint16Len:]), true
}

func NumChunks(value []byte) (uint16, bool) {
	l := len(value)
	return numChunks(l)
}

func numChunks(valueLen int) (uint16, bool) {
	if valueLen == 0 {
		return 0, true
	}
	raw := valueLen/chunkSize + 1
	if raw > int(consts.MaxUint16) {
		return 0, false
	}
	return uint16(raw), true
}

func Verify(maxKeySize uint32, maxValueChunks uint16, key string) bool {
	if uint32(len(key)) > maxKeySize {
		return false
	}
	keyChunks, ok := MaxChunks(key)
	if !ok {
		return false
	}
	return keyChunks <= maxValueChunks
}

func VerifyValue(key string, value []byte) bool {
	valueChunks, ok := NumChunks(value)
	if !ok {
		return false
	}
	keyChunks, ok := MaxChunks(key)
	if !ok {
		return false
	}
	return valueChunks <= keyChunks
}

func Encode(key string, maxSize int) (string, bool) {
	numChunks, ok := numChunks(maxSize)
	if !ok {
		return "", false
	}
	bkey := []byte(key)
	bkey = binary.BigEndian.AppendUint16(bkey, numChunks)
	return string(bkey), true
}

func EncodeChunks(key string, maxChunks uint16) string {
	bkey := []byte(key)
	bkey = binary.BigEndian.AppendUint16(bkey, maxChunks)
	return string(bkey)
}
