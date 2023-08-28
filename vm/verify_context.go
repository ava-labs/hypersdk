package vm

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.VerifyContext = (*AcceptedVerifyContext)(nil)
var _ chain.VerifyContext = (*PendingVerifyContext)(nil)

func (vm *VM) GetVerifyContext(ctx context.Context, blockHeight uint64, parent ids.ID) (chain.VerifyContext, error) {
	// If last accepted block is not processed, we need to process it. This will happen when we call [View] on the parent.
	if blockHeight != 0 && ((!vm.lastAccepted.Processed() && parent == vm.lastAccepted.ID()) || blockHeight-1 > vm.lastAccepted.Hght) {
		blk, err := vm.GetStatelessBlock(ctx, parent)
		if err != nil {
			return nil, err
		}
		return &PendingVerifyContext{blk}, nil
	}
	return &AcceptedVerifyContext{vm}, nil
}

type PendingVerifyContext struct {
	blk *chain.StatelessBlock
}

func (p *PendingVerifyContext) View(ctx context.Context, blockRoot ids.ID) (state.View, error) {
	return p.blk.View(ctx, &blockRoot)
}

func (p *PendingVerifyContext) IsRepeat(ctx context.Context, oldestAllowed int64, txs []*chain.Transaction, marker set.Bits, stop bool) (set.Bits, error) {
	return p.blk.IsRepeat(ctx, oldestAllowed, txs, marker, stop)
}

type AcceptedVerifyContext struct {
	vm *VM
}

// The caller of this should check for equality with the root during verification.
func (a *AcceptedVerifyContext) View(ctx context.Context, _ ids.ID) (state.View, error) {
	return a.vm.State()
}

func (a *AcceptedVerifyContext) IsRepeat(ctx context.Context, oldestAllowed int64, txs []*chain.Transaction, marker set.Bits, stop bool) (set.Bits, error) {
	bits := a.vm.IsRepeat(ctx, txs, marker, stop)
	return bits, nil
}
