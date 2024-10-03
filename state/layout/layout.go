// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package layout

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/keys"
)

const (
	balanceKeyChunks uint16 = 1

	heightKeyChunks    = 1
	timestampKeyChunks = 1
	feeKeyChunks       = 8 // 96 (per dimension) * 5 (num dimensions)
)

const (
	DefaultHeightPrefix byte = iota
	DefaultTimestampPrefix
	DefaultFeePrefix

	DefaultBalancePrefix
	DefaultActionPrefix
)

// Layout defines hypersdk-manged state keys
type Layout struct {
	heightKey        []byte
	timestampKey     []byte
	feeKey           []byte
	balanceKeyPrefix []byte
	actionPrefix     []byte
}

func New(
	heightPrefix []byte,
	timestampPrefix []byte,
	feePrefix []byte,
	balanceKeyPrefix []byte,
	actionKeyPrefix []byte,
) Layout {
	return Layout{
		heightKey:        keys.EncodeChunks(heightPrefix, heightKeyChunks),
		timestampKey:     keys.EncodeChunks(timestampPrefix, timestampKeyChunks),
		feeKey:           keys.EncodeChunks(feePrefix, feeKeyChunks),
		balanceKeyPrefix: balanceKeyPrefix,
		actionPrefix:     actionKeyPrefix,
	}
}

func Default() Layout {
	return New(
		[]byte{DefaultHeightPrefix},
		[]byte{DefaultTimestampPrefix},
		[]byte{DefaultFeePrefix},
		[]byte{DefaultBalancePrefix},
		[]byte{DefaultActionPrefix},
	)
}

func (l Layout) HeightKey() []byte {
	return l.heightKey
}

func (l Layout) TimestampKey() []byte {
	return l.timestampKey
}

func (l Layout) FeeKey() []byte {
	return l.feeKey
}

func (l Layout) NewBalanceKey(address codec.Address) []byte {
	return newKeyWithPrefix(l.balanceKeyPrefix, address[:], balanceKeyChunks)
}

func (l Layout) NewActionKey(key []byte, chunks uint16) []byte {
	return newKeyWithPrefix(l.actionPrefix, key, chunks)
}

func newKeyWithPrefix(prefix []byte, key []byte, chunks uint16) []byte {
	k := make([]byte, 0)
	k = append(k, prefix...)
	k = append(k, key...)

	return keys.EncodeChunks(k, chunks)
}
