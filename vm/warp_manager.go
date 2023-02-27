package vm

import (
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/hypersdk/codec"
	"go.uber.org/zap"
)

const maxWarpResponse = bls.PublicKeyLen + bls.SignatureLen

// WarpManager takes requests to get signatures from other nodes and then
// stores the result in our DB for future usage.
type WarpManager struct {
	vm *VM

	l         sync.Mutex
	requestID uint32

	jobs map[uint32]*signatureJob
}

type signatureJob struct {
	nodeID  ids.NodeID
	txID    ids.ID
	retries int
	msg     []byte
}

func NewWarpManager(vm *VM) *WarpManager {
	return &WarpManager{
		vm:   vm,
		jobs: map[uint32]*signatureJob{},
	}
}

// GatherSignatures makes a best effort to acquire signatures from other
// validators and store them inside the vmDB.
//
// This function is blocking and it is typically suggested that this be done in
// the background.
func (w *WarpManager) GatherSignatures(ctx context.Context, txID ids.ID, msg []byte) {
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
	// TODO: restart any unfinished jobs on restart
	for nodeID, validator := range validators {
		// Make request to validator over p2p (retry x times)

		// Check signature validity

		// Store in DB
	}
}

func (w *WarpManager) Request(ctx context.Context, nodeID ids.NodeID, txID ids.ID) {
	w.l.Lock()
	requestID := w.requestID
	w.requestID++
	w.l.Unlock()
}

func (w *WarpManager) HandleResponse(requestID uint32, msg []byte) {
	w.l.Lock()
	job, ok := w.jobs[requestID]
	if ok {
		delete(w.jobs, requestID)
	}
	w.l.Unlock()
	if !ok {
		return
	}

	// Parse message
	r := codec.NewReader(msg, maxWarpResponse)
	var publicKey []byte
	r.UnpackBytes(bls.PublicKeyLen, true, &publicKey)
	var signature []byte
	r.UnpackBytes(bls.SignatureLen, true, &signature)
	if err := r.Err(); err != nil {
		return
	}

	// Check signature validity
	pk, err := bls.PublicKeyFromBytes(publicKey)
	if err != nil {
		return
	}
	sig, err := bls.SignatureFromBytes(signature)
	if err != nil {
		return
	}
	if !bls.Verify(pk, sig, job.msg) {
		return
	}

	// Store in DB
	if err := w.vm.StoreWarpSignature(job.txID, pk, signature); err != nil {
		return
	}
}

func (w *WarpManager) HandleRequestFailed() {
	// TODO: retry 5 times (after waiting 30s)
}
