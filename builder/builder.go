// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

type Builder interface {
	Run()
	TriggerBuild()
	HandleGenerateBlock()
	Done() // wait after stop
}
