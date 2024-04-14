package vilmo

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
)

const (
	opPut      = uint8(0) // keyLen|key|valueLen|value
	opDelete   = uint8(1) // keyLen|key
	opBatch    = uint8(2) // batch (starts all segments)
	opChecksum = uint8(3) // checksum (ends all segments)
	opNullify  = uint8(4) // keyLoc
)

func readOpType(reader *reader, hasher hash.Hash) (uint8, error) {
	op := make([]byte, consts.Uint8Len)
	if err := reader.Read(op); err != nil {
		return 0, err
	}
	if _, err := hasher.Write(op); err != nil {
		return 0, err
	}
	return op[0], nil
}

func readKey(reader *reader, hasher hash.Hash) (string, error) {
	op := make([]byte, consts.Uint16Len)
	if err := reader.Read(op); err != nil {
		return "", err
	}
	if _, err := hasher.Write(op); err != nil {
		return "", err
	}
	keyLen := binary.BigEndian.Uint16(op)
	key := make([]byte, keyLen)
	if err := reader.Read(key); err != nil {
		return "", err
	}
	if _, err := hasher.Write(key); err != nil {
		return "", err
	}
	return string(key), nil
}

func readPut(reader *reader, hasher hash.Hash) (string, []byte, error) {
	key, err := readKey(reader, hasher)
	if err != nil {
		return "", nil, err
	}

	// Read value
	op := make([]byte, consts.Uint32Len)
	if err := reader.Read(op); err != nil {
		return "", nil, err
	}
	if _, err := hasher.Write(op); err != nil {
		return "", nil, err
	}
	valueLen := binary.BigEndian.Uint32(op)
	value := make([]byte, valueLen)
	if err := reader.Read(value); err != nil {
		return "", nil, err
	}
	if _, err := hasher.Write(value); err != nil {
		return "", nil, err
	}
	return key, value, nil
}

func readDelete(reader *reader, hasher hash.Hash) (string, error) {
	key, err := readKey(reader, hasher)
	if err != nil {
		return "", err
	}
	return key, nil
}

func readBatch(reader *reader, hasher hash.Hash) (uint64, error) {
	op := make([]byte, consts.Uint64Len)
	if err := reader.Read(op); err != nil {
		return 0, err
	}
	if _, err := hasher.Write(op); err != nil {
		return 0, err
	}
	batch := binary.BigEndian.Uint64(op)
	return batch, nil
}

func readChecksum(reader *reader) (ids.ID, error) {
	op := make([]byte, sha256.Size)
	if err := reader.Read(op); err != nil {
		return ids.Empty, err
	}
	return ids.ID(op), nil
}

func readNullify(reader *reader, hasher hash.Hash) (int64, error) {
	op := make([]byte, consts.Uint64Len)
	if err := reader.Read(op); err != nil {
		return 0, err
	}
	if _, err := hasher.Write(op); err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(op)), nil
}

func opPutLen(key string, value []byte) int64 {
	return int64(consts.Uint8Len + consts.Uint16Len + len(key) + consts.Uint32Len + len(value))
}

func opPutLenWithValueLen(key string, valueLen int64) int64 {
	return int64(consts.Uint8Len+consts.Uint16Len+len(key)+consts.Uint32Len) + valueLen
}

func opPutToValue(keyLen uint16) int64 {
	return int64(consts.Uint8Len + consts.Uint16Len + keyLen + consts.Uint32Len)
}

func opDeleteLen(key string) int64 {
	return int64(consts.Uint8Len + consts.Uint16Len + len(key))
}

func opBatchLen() int64 {
	return int64(consts.Uint8Len + consts.Uint64Len)
}

func opChecksumLen() int64 {
	return int64(consts.Uint8Len + ids.IDLen)
}

func opNullifyLen() int64 {
	return int64(consts.Uint8Len + consts.Uint64Len)
}
