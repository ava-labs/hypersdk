package vm

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"go.uber.org/zap"
)

// WarpManager takes requests to get signatures from other nodes and then
// stores the result in our DB for future usage.
type WarpManager struct {
	vm *VM
}

func NewWarpManager(vm *VM) *WarpManager {
	return &WarpManager{vm}
}

// GatherSignatures makes a best effort to acquire signatures from other
// validators and store them inside the vmDB.
//
// This function is blocking and it is typically suggested that this be done in
// the background.
func (w *WarpManager) GatherSignatures(ctx context.Context, txID ids.ID) {
	height, err := w.vm.snowCtx.ValidatorState.GetCurrentHeight(ctx)
	if err != nil {
		w.vm.snowCtx.Log.Error("unable to get current p-chain height", zap.Error(err))
		return
	}
	validators, err := w.vm.snowCtx.ValidatorState.GetValidatorSet(
		ctx,
		height,
		w.vm.snowCtx.SubnetID,
	)
	if err != nil {
		w.vm.snowCtx.Log.Error("unable to get validator set", zap.Error(err))
		return
	}
	// TODO: run this fetch in parallel (limit to 4 at a time)
	// TODO: exit if vm is stopped
	// TODO: restart any unfinished jobs in the future
	for nodeID, validator := range validators {
		// Make request to validator over p2p (retry x times)

		// Check signature validity

		// Store in DB
	}
}
