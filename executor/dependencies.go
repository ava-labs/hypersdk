// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

type Metrics interface {
	RecordBlocked()
	RecordExecutable()
}
