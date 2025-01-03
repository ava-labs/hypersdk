// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tieredstorage

import "errors"

var (
	errFailedToParseMaxChunks = errors.New("failed to parse max chunks")
	errValueTooShortForSuffix = errors.New("value is too short to contain suffix")
	errFailedToRefund         = errors.New("failed to refund units consumed")
)
