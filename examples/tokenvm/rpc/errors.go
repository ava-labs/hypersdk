// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import "errors"

var (
	ErrTxNotFound    = errors.New("tx not found")
	ErrAssetNotFound = errors.New("asset not found")
)
