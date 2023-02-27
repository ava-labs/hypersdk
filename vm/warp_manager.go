package vm

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"go.uber.org/zap"
)

const maxWarpResponse = bls.PublicKeyLen + bls.SignatureLen

// WarpManager takes requests to get signatures from other nodes and then
// stores the result in our DB for future usage.
//
// TODO: limit max requests outstanding at any time
type WarpManager struct {
	vm        *VM
	appSender common.AppSender

	l         sync.Mutex
	requestID uint32

	jobs map[uint32]*signatureJob
}

type signatureJob struct {
	nodeID    ids.NodeID
	publicKey []byte
	txID      ids.ID
	retry     int
	msg       []byte
}

func NewWarpManager(vm *VM, appSender common.AppSender) *WarpManager {
	return &WarpManager{
		vm:        vm,
		appSender: appSender,
		jobs:      map[uint32]*signatureJob{},
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
		// Only request from validators that have registered BLS public keys
		if validator.PublicKey == nil {
			continue
		}
		w.Request(ctx, nodeID, bls.PublicKeyToBytes(validator.PublicKey), txID, 0, msg)
	}
}

func (w *WarpManager) Request(
	ctx context.Context,
	nodeID ids.NodeID,
	publicKey []byte,
	txID ids.ID,
	retry int,
	msg []byte,
) error {

	w.l.Lock()
	requestID := w.requestID
	w.requestID++
	w.jobs[requestID] = &signatureJob{
		nodeID:    nodeID,
		publicKey: publicKey,
		txID:      txID,
		retry:     retry,
		msg:       msg,
	}
	w.l.Unlock()
	p := codec.NewWriter(consts.IDLen)
	p.PackID(txID)
	if err := p.Err(); err != nil {
		// Should never happen
		w.l.Lock()
		delete(w.jobs, requestID)
		w.l.Unlock()
		return nil
	}
	return w.appSender.SendAppRequest(
		ctx,
		set.Set[ids.NodeID]{nodeID: struct{}{}},
		requestID,
		p.Bytes(),
	)
}

func (w *WarpManager) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	request []byte,
) error {
	rp := codec.NewReader(request, consts.IDLen)
	var txID ids.ID
	rp.UnpackID(true, &txID)
	if err := rp.Err(); err != nil {
		return nil
	}
	sig, err := w.vm.GetWarpSignature(txID, w.vm.snowCtx.PublicKey)
	if err != nil {
		return nil
	}
	if sig == nil {
		return nil
	}
	wp := codec.NewWriter(maxWarpResponse)
	wp.PackBytes(sig.PublicKey)
	wp.PackBytes(sig.Signature)
	if err := wp.Err(); err != nil {
		return nil
	}
	return w.appSender.SendAppResponse(ctx, nodeID, requestID, wp.Bytes())
}

func (w *WarpManager) HandleResponse(requestID uint32, msg []byte) error {
	w.l.Lock()
	job, ok := w.jobs[requestID]
	delete(w.jobs, requestID)
	w.l.Unlock()
	if !ok {
		return nil
	}

	// Parse message
	r := codec.NewReader(msg, maxWarpResponse)
	var publicKey []byte
	r.UnpackBytes(bls.PublicKeyLen, true, &publicKey)
	var signature []byte
	r.UnpackBytes(bls.SignatureLen, true, &signature)
	if err := r.Err(); err != nil {
		return nil
	}

	// Check public key is expected
	if !bytes.Equal(publicKey, job.publicKey) {
		return nil
	}

	// Check signature validity
	pk, err := bls.PublicKeyFromBytes(publicKey)
	if err != nil {
		return nil
	}
	sig, err := bls.SignatureFromBytes(signature)
	if err != nil {
		return nil
	}
	if !bls.Verify(pk, sig, job.msg) {
		return nil
	}

	// Store in DB
	if err := w.vm.StoreWarpSignature(job.txID, pk, signature); err != nil {
		return nil
	}
	return nil
}

func (w *WarpManager) HandleRequestFailed(requestID uint32) error {
	w.l.Lock()
	job, ok := w.jobs[requestID]
	delete(w.jobs, requestID)
	w.l.Unlock()
	if !ok {
		return nil
	}
	if job.retry >= 5 {
		return nil
	}

	timer := time.NewTimer(30 * time.Second)
	select {
	case <-timer.C:
	case <-w.vm.stop:
		return nil
	}

	return w.Request(context.TODO(), job.nodeID, job.publicKey, job.txID, job.retry+1, job.msg)
}
