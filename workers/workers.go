// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workers

type Workers interface {
	NewJob(backlog int) (Job, error)
	Stop()
}

type Job interface {
	Go(func() error)
	Done(func())
	Wait() error
	Workers() int
}
