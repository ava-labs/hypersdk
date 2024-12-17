// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"encoding/binary"
	"errors"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/keys"
)

var (
	ErrValueTooShortForSuffix = errors.New("value too short for suffix")
	ErrFailedToParseMaxChunks = errors.New("failed to parse max chunks")
)

func ExtractSuffixes(storage map[string][]byte, blockHeight uint64, epsilon uint64) (
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
