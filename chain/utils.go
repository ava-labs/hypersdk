// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"encoding/binary"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/utils"
)

func CreateActionID(txID ids.ID, i uint8) ids.ID {
	actionBytes := make([]byte, ids.IDLen+consts.Uint8Len)
	copy(actionBytes, txID[:])
	actionBytes[ids.IDLen] = i
	return utils.ToID(actionBytes)
}

func extractSuffixes(storage map[string][]byte, blockHeight uint64, epsilon uint64) (
	map[string][]byte,
	map[string]uint16,
	error,
) {
	unsuffixedStorage := make(map[string][]byte)
	hotkeys := make(map[string]uint16)

	for k, v := range storage {
		if len(v) < consts.Uint64Len {
			return nil, nil, ErrValueTooShortForSuffix
		}
		lastTouched := binary.BigEndian.Uint64(v[len(v)-consts.Uint64Len:])

		// If hot key
		if blockHeight-lastTouched <= epsilon {
			maxChunks, ok := keys.MaxChunks([]byte(k))
			if !ok {
				return nil, nil, ErrFailedToParseMaxChunks
			}
			hotkeys[k] = maxChunks
		}

		unsuffixedStorage[k] = v[:len(v)-consts.Uint64Len]
	}

	return unsuffixedStorage, hotkeys, nil
}
