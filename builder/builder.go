// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

type Builder interface {
	Run()
	QueueNotify() // new tx, block verified, post-block build (if mempool > 0)
	ForceNotify()
	Done() // wait after stop
}
