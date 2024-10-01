// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/ava-labs/avalanchego/utils/set"
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

	// heightKeyName    = "height"
	// timestampKeyName = "timestamp"
	// feeKeyName       = "fee"
)

// TODO test user writing to managed key (balance prefix + account collides with
// reserved key)
func NewLayout(
	heightPrefix byte,
	timestampPrefix byte,
	feePrefix byte,
	vmSpecificPrefixes []byte,
) (Layout, error) {

	prefixes := []byte{
		heightPrefix,
		timestampPrefix,
		feePrefix,
	}

	prefixes = append(prefixes, vmSpecificPrefixes...)

	verifiedPrefixes := set.Set[string]{}

	for _, k := range prefixes {
		keyString := string(k)

		for prefix := range verifiedPrefixes {
			if !strings.HasPrefix(prefix, keyString) {
				return Layout{}, fmt.Errorf("invalid state key %s: %w", string(k), ErrConflictingKey)
			}
		}

		verifiedPrefixes.Add(keyString)
	}

	return Layout{
		heightKey:        keys.EncodeChunks([]byte{heightPrefix}, heightKeyChunks),
		timestampKey:     keys.EncodeChunks([]byte{timestampPrefix}, timestampKeyChunks),
		feeKey:           keys.EncodeChunks([]byte{feePrefix}, feeKeyChunks),
	}, nil
}

// Layout defines hypersdk-manged state keys
// TODO unit tests
// TODO rename Schema/Factory/Keys
type Layout struct {
	heightKey        []byte
	timestampKey     []byte
	feeKey           []byte
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

func (l Layout) ConflictingPrefix(prefix byte) bool {
	return bytes.HasPrefix(l.heightKey, []byte{prefix}) ||
		bytes.HasPrefix(l.timestampKey, []byte{prefix}) ||
		bytes.HasPrefix(l.feeKey, []byte{prefix})
}
