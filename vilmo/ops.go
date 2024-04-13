package vilmo

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"io"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
)

const (
	opPut      = uint8(0) // keyLen|key|valueLen|value
	opDelete   = uint8(1) // keyLen|key
	opBatch    = uint8(2) // batch (starts all segments)
	opChecksum = uint8(3) // checksum (ends all segments)
)

type op interface {
	Type() uint8
}

type putOp struct {
	key   string
	value []byte
}

func (op *putOp) Type() uint8 {
	return opPut
}

type deleteOp struct {
	key string
}

func (op *deleteOp) Type() uint8 {
	return opDelete
}

type batchOp struct {
	batch uint64
}

func (op *batchOp) Type() uint8 {
	return opBatch
}

type checksumOp struct {
	checksum ids.ID
}

func (op *checksumOp) Type() uint8 {
	return opChecksum
}

func readOpType(reader io.Reader, hasher hash.Hash) (uint8, error) {
	op := make([]byte, consts.Uint8Len)
	if _, err := io.ReadFull(reader, op); err != nil {
		return 0, err
	}
	if _, err := hasher.Write(op); err != nil {
		return 0, err
	}
	return op[0], nil
}

func readKey(reader io.Reader, hasher hash.Hash) (string, error) {
	op := make([]byte, consts.Uint16Len)
	if _, err := io.ReadFull(reader, op); err != nil {
		return "", err
	}
	if _, err := hasher.Write(op); err != nil {
		return "", err
	}
	keyLen := binary.BigEndian.Uint16(op)
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(reader, key); err != nil {
		return "", err
	}
	if _, err := hasher.Write(key); err != nil {
		return "", err
	}
	return string(key), nil
}

func readPut(reader io.Reader, hasher hash.Hash) (*putOp, error) {
	key, err := readKey(reader, hasher)
	if err != nil {
		return nil, err
	}

	// Read value
	op := make([]byte, consts.Uint32Len)
	if _, err := io.ReadFull(reader, op); err != nil {
		return nil, err
	}
	if _, err := hasher.Write(op); err != nil {
		return nil, err
	}
	valueLen := binary.BigEndian.Uint32(op)
	value := make([]byte, valueLen)
	if _, err := io.ReadFull(reader, value); err != nil {
		return nil, err
	}
	if _, err := hasher.Write(value); err != nil {
		return nil, err
	}
	return &putOp{key, value}, nil
}

func readDelete(reader io.Reader, hasher hash.Hash) (*deleteOp, error) {
	key, err := readKey(reader, hasher)
	if err != nil {
		return nil, err
	}
	return &deleteOp{key}, nil
}

func readBatch(reader io.Reader, cursor int64, hasher hash.Hash) (*batchOp, error) {
	op := make([]byte, consts.Uint64Len)
	if _, err := io.ReadFull(reader, op); err != nil {
		return nil, err
	}
	if _, err := hasher.Write(op); err != nil {
		return nil, err
	}
	cursor += int64(len(op))
	batch := binary.BigEndian.Uint64(op)
	return &batchOp{batch}, nil
}

func readChecksum(reader io.Reader) (*checksumOp, error) {
	op := make([]byte, sha256.Size)
	if _, err := io.ReadFull(reader, op); err != nil {
		return nil, err
	}
	return &checksumOp{ids.ID(op)}, nil
}

func opPutLen(key string, value []byte) int64 {
	return int64(consts.Uint8Len + consts.Uint16Len + len(key) + consts.Uint32Len + len(value))
}

func opPutLenWithValueLen(key string, valueLen int64) int64 {
	return int64(consts.Uint8Len+consts.Uint16Len+len(key)+consts.Uint32Len) + valueLen
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
