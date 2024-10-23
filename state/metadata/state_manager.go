// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package metadata

import (
	"bytes"

	"github.com/ava-labs/hypersdk/chain"
)

const (
	defaultHeightPrefix    byte = 0x0
	defaultTimestampPrefix byte = 0x1
	defaultFeePrefix       byte = 0x2

	DefaultMinimumPrefix byte = 0x3
)

var _ chain.MetadataManager = (*MetadataManager)(nil)

type MetadataManager struct {
	heightPrefix    []byte
	feePrefix       []byte
	timestampPrefix []byte
}

func NewManager(
	heightPrefix []byte,
	feePrefix []byte,
	timestampPrefix []byte,
) MetadataManager {
	return MetadataManager{
		heightPrefix:    heightPrefix,
		feePrefix:       feePrefix,
		timestampPrefix: timestampPrefix,
	}
}

func NewDefaultManager() MetadataManager {
	return MetadataManager{
		heightPrefix:    []byte{defaultHeightPrefix},
		feePrefix:       []byte{defaultFeePrefix},
		timestampPrefix: []byte{defaultTimestampPrefix},
	}
}

func (m MetadataManager) FeePrefix() []byte {
	return m.feePrefix
}

func (m MetadataManager) HeightPrefix() []byte {
	return m.heightPrefix
}

func (m MetadataManager) TimestampPrefix() []byte {
	return m.timestampPrefix
}

// Returns true if all prefixes in `m` and `vmPrefixes` are unique
func HasConflictingPrefixes(
	m chain.MetadataManager,
	vmPrefixes [][]byte,
) bool {
	prefixes := [][]byte{
		m.HeightPrefix(),
		m.FeePrefix(),
		m.TimestampPrefix(),
	}

	prefixes = append(prefixes, vmPrefixes...)
	verifiedPrefixes := make([][]byte, 0)

	for _, p := range prefixes {
		for _, vp := range verifiedPrefixes {
			if bytes.HasPrefix(p, vp) || bytes.HasPrefix(vp, p) {
				return true
			}
		}

		verifiedPrefixes = append(verifiedPrefixes, p)
	}

	return false
}
