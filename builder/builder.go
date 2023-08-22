// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

type Builder interface {
	Run()
	Queue() // new tx, block verified, post-block build (if mempool > 0)
	Force()
	Done() // wait after stop
}
