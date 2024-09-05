// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/keys"
)

const (
	heightKeyChunks           = 1
	timestampKeyChunks        = 1
	feeKeyChunks              = 8 // 96 (per dimension) * 5 (num dimensions)
	balanceKeyChunks   uint16 = 1
)

var (
	//TODO fix errors
	ErrInvalidKey       = errors.New("invalid key")
	ErrInvalidKeyPrefix = errors.New("invalid key prefix")
	ErrDuplicateKey     = errors.New("duplicate key")
	ErrConflictingKey   = errors.New("conflicting key")

	heightKeyName    = "height"
	timestampKeyName = "timestamp"
	feeKeyName       = "fee"
)

// TODO test user writing to managed key (balance prefix + account collides with
// reserved key)
func NewLayout(
	heightKey []byte,
	timestampKey []byte,
	feeKey []byte,
	balanceKeyPrefix []byte,
	actionKeyPrefix []byte,
) Layout {
	return Layout{
		heightKey:        keys.EncodeChunks(heightKey, heightKeyChunks),
		timestampKey:     keys.EncodeChunks(timestampKey, timestampKeyChunks),
		feeKey:           keys.EncodeChunks(feeKey, feeKeyChunks),
		balanceKeyPrefix: balanceKeyPrefix,
		actionPrefix:     actionKeyPrefix,
	}
}

// Layout defines hypersdk-manged state keys
// TODO unit tests
// TODO rename Schema/Factory/Keys
type Layout struct {
	heightKey        []byte
	timestampKey     []byte
	feeKey           []byte
	balanceKeyPrefix []byte
	actionPrefix     []byte
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
	k := make([]byte, len(prefix)+len(key))
	k = append(k, prefix...)
	k = append(k, key[:]...)

	return keys.EncodeChunks(k, chunks)
}

func (l Layout) Verify() error {
	prefixes := [][]byte{
		l.heightKey,
		l.timestampKey,
		l.feeKey,
		l.balanceKeyPrefix,
		l.actionPrefix,
	}
	verifiedPrefixes := set.Set[string]{}

	for _, k := range prefixes {
		keyString := string(k)

		for prefix := range verifiedPrefixes {
			if !strings.HasPrefix(prefix, keyString) {
				return fmt.Errorf("invalid state key %s: %w", k, ErrConflictingKey)
			}
		}

		verifiedPrefixes.Add(keyString)
	}

	return nil
}
