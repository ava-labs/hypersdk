// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"github.com/ava-labs/hypersdk/state/metadata"
)

const (
	accountPrefix byte = metadata.DefaultMinimumPrefix + iota
	storagePrefix
	codePrefix

	StorageChunks = 1
	AccountChunks = 2
	CodeChunks    = 1024 * 24 / 64 // the max contract size is 24KB
)
