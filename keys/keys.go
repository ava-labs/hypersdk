package keys

import (
	"encoding/binary"

	"github.com/ava-labs/hypersdk/consts"
)

const chunkSize = 64 // bytes

func MaxChunks(key []byte) (uint16, bool) {
	l := len(key)
	if l < consts.Uint16Len {
		return 0, false
	}
	return binary.BigEndian.Uint16(key[l-consts.Uint16Len:]) * chunkSize, true
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

func Verify(maxKeySize uint32, maxValueChunks uint16, key []byte) bool {
	if uint32(len(key)) > maxKeySize {
		return false
	}
	keyChunks, ok := MaxChunks(key)
	if !ok {
		return false
	}
	return keyChunks <= maxValueChunks
}

func VerifyValue(maxKeySize uint32, maxValueChunks uint16, key []byte, value []byte) bool {
	if uint32(len(key)) > maxKeySize {
		return false
	}
	valueChunks, ok := NumChunks(value)
	if !ok {
		return false
	}
	if valueChunks > maxValueChunks {
		return false
	}
	keyChunks, ok := MaxChunks(key)
	if !ok {
		return false
	}
	return valueChunks <= keyChunks
}

func Encode(key []byte, maxSize int) ([]byte, bool) {
	numChunks, ok := numChunks(maxSize)
	if !ok {
		return nil, false
	}
	return binary.BigEndian.AppendUint16(key, numChunks), true
}

func EncodeChunks(key []byte, maxChunks uint16) []byte {
	return binary.BigEndian.AppendUint16(key, maxChunks)
}
