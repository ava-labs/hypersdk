package loadgen

import (
	"encoding/binary"
)

func uniqueBytes() []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(sent.Add(1)))
}
