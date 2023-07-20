// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import "github.com/ava-labs/avalanchego/snow/engine/common"

var _ Builder = (*Manual)(nil)

type Manual struct {
	vm        VM
	doneBuild chan struct{}
}

func NewManual(vm VM) *Manual {
	return &Manual{
		vm:        vm,
		doneBuild: make(chan struct{}),
	}
}

func (b *Manual) Run() {
	close(b.doneBuild)
}

func (*Manual) QueueNotify() {}

func (b *Manual) ForceNotify() {
	select {
	case b.vm.EngineChan() <- common.PendingTxs:
	default:
		b.vm.Logger().Debug("dropping message to consensus engine")
	}
}

func (b *Manual) Done() {
	<-b.doneBuild
}
