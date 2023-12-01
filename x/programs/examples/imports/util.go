package imports

import (
	"encoding/binary"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

// grabBytesFromPtr returns bytes from memory at [ptr].
func GrabBytesFromPtr(client runtime.WasmtimeExportClient, ptr int64) ([]byte, error) {
	memory := runtime.NewMemory(client)

	// grab the first 4 bytes from ptr which is the length of bytes
	lenBytes, err := memory.Range(uint64(ptr), consts.Uint32Len)
	if err != nil {
		return nil, err
	}

	// convert to uint32
	length := binary.BigEndian.Uint32(lenBytes)

	// grab the remaining bytes from ptr which is the actual bytes
	bytes, err := memory.Range(uint64(ptr + consts.Uint32Len), uint64(length))
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func PrependLength(bytes []byte) []byte {
	length := uint32(len(bytes))
	lenBytes := make([]byte, consts.Uint32Len)
	binary.BigEndian.PutUint32(lenBytes, length)
	return append(lenBytes, bytes...)
}