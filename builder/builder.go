// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import "context"

type Builder interface {
	Run()
	Queue(context.Context) // new tx, block verified, post-block build (if mempool > 0)
	Force(context.Context) error
	Done() // wait after stop
}
