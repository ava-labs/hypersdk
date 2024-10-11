// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"encoding/binary"
)

func uniqueBytes() []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(sent.Add(1)))
}
