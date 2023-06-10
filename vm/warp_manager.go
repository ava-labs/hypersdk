// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"bytes"
	"context"
	"encoding/hex"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/heap"
	"github.com/AnomalyFi/hypersdk/utils"
	"go.uber.org/zap"
)

const (
	maxWarpResponse   = bls.PublicKeyLen + bls.SignatureLen
	minGatherInterval = 30 * 60 // 30 minutes
	initialBackoff    = 2       // give time for others to sign
	backoffIncrease   = 5
	maxRetries        = 10
	maxOutstanding    = 8 // TODO: make a config
)

// WarpManager takes requests to get signatures from other nodes and then
// stores the result in our DB for future usage.
type WarpManager struct {
	vm        *VM
	appSender common.AppSender

	l         sync.Mutex
	requestID uint32

	pendingJobs *heap.Heap[*signatureJob, int64]
	jobs        map[uint32]*signatureJob

	done chan struct{}
}

type signatureJob struct {
	id        ids.ID
	nodeID    ids.NodeID
	publicKey []byte
	txID      ids.ID
	retry     int
	msg       []byte
}

func NewWarpManager(vm *VM) *WarpManager {
	return &WarpManager{
		vm:          vm,
		pendingJobs: heap.New[*signatureJob, int64](64, true),
		jobs:        map[uint32]*signatureJob{},
		done:        make(chan struct{}),
	}
}

func (w *WarpManager) Run(appSender common.AppSender) {
	w.appSender = appSender

	w.vm.Logger().Info("starting warp manager")
	defer close(w.done)

	t := time.NewTicker(1 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			w.l.Lock()
			now := time.Now().Unix()
			for w.pendingJobs.Len() > 0 && len(w.jobs) < maxOutstanding {
				first := w.pendingJobs.First()
				if first.Val > now {
					break
				}
				w.pendingJobs.Pop()

				// Send request
				job := first.Item
				if err := w.request(context.Background(), job); err != nil {
					w.vm.snowCtx.Log.Error(
						"unable to request signature",
						zap.Stringer("nodeID", job.nodeID),
						zap.Error(err),
					)
				}
			}
			l := w.pendingJobs.Len()
			w.l.Unlock()
			w.vm.snowCtx.Log.Debug("checked for ready jobs", zap.Int("pending", l))
		case <-w.vm.stop:
			w.vm.Logger().Info("stopping warp manager")
			return
		}
	}
}

// GatherSignatures makes a best effort to acquire signatures from other
// validators and store them inside the vmDB.
//
// GatherSignatures may be called when a block is accepted (optimistically) or
// may be triggered by RPC (if missing signatures are detected). To prevent RPC
// abuse, we limit how frequently we attempt to gather signatures for a given
// TxID.
func (w *WarpManager) GatherSignatures(ctx context.Context, txID ids.ID, msg []byte) {
	lastFetch, err := w.vm.GetWarpFetch(txID)
	if err != nil {
		w.vm.snowCtx.Log.Error("unable to get last fetch", zap.Error(err))
		return
	}
	if time.Now().Unix()-lastFetch < minGatherInterval {
		w.vm.snowCtx.Log.Error("skipping fetch too recent", zap.Stringer("txID", txID))
		return
	}
	if err := w.vm.StoreWarpFetch(txID); err != nil {
		w.vm.snowCtx.Log.Error("unable to get last fetch", zap.Error(err))
		return
	}
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
	for nodeID, validator := range validators {
		// Only request from validators that have registered BLS public keys and
		// that we have not already gotten a signature from.
		if validator.PublicKey == nil {
			w.vm.snowCtx.Log.Info(
				"skipping fetch for validator with no registered public key",
				zap.Stringer("nodeID", nodeID),
				zap.Uint64("pchain height", height),
			)
			continue
		}
		previousSignature, err := w.vm.GetWarpSignature(txID, validator.PublicKey)
		if err != nil {
			w.vm.snowCtx.Log.Error("unable to fetch previous signature", zap.Error(err))
			return
		}
		if previousSignature != nil {
			continue
		}

		idb := make([]byte, consts.IDLen+consts.NodeIDLen)
		copy(idb, txID[:])
		copy(idb[consts.IDLen:], nodeID.Bytes())
		id := utils.ToID(idb)
		w.l.Lock()
		if w.pendingJobs.Has(id) {
			// We may already have enqueued a job when the block was accepted.
			w.l.Unlock()
			continue
		}
		w.pendingJobs.Push(&heap.Entry[*signatureJob, int64]{
			ID: id,
			Item: &signatureJob{
				id,
				nodeID,
				bls.PublicKeyToBytes(validator.PublicKey),
				txID,
				0,
				msg,
			},
			Val:   time.Now().Unix() + initialBackoff,
			Index: w.pendingJobs.Len(),
		})
		w.l.Unlock()
		w.vm.snowCtx.Log.Debug(
			"enqueued fetch job",
			zap.Stringer("nodeID", nodeID),
			zap.Stringer("txID", txID),
		)
	}
}

// you must hold [w.l] when calling this function
func (w *WarpManager) request(
	ctx context.Context,
	j *signatureJob,
) error {
	requestID := w.requestID
	w.requestID++
	w.jobs[requestID] = j

	p := codec.NewWriter(consts.IDLen)
	p.PackID(j.txID)
	if err := p.Err(); err != nil {
		// Should never happen
		delete(w.jobs, requestID)
		return nil
	}
	return w.appSender.SendAppRequest(
		ctx,
		set.Set[ids.NodeID]{j.nodeID: struct{}{}},
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
		w.vm.snowCtx.Log.Warn("unable to unpack request", zap.Error(err))
		return nil
	}
	sig, err := w.vm.GetWarpSignature(txID, w.vm.snowCtx.PublicKey)
	if err != nil {
		w.vm.snowCtx.Log.Warn("could not fetch warp signature", zap.Error(err))
		return nil
	}
	if sig == nil {
		// Generate and save signature if it does not exist but is in state (may
		// have been offline when message was accepted)
		msg, err := w.vm.GetOutgoingWarpMessage(txID)
		if msg == nil || err != nil {
			w.vm.snowCtx.Log.Warn("could not get outgoing warp message", zap.Error(err))
			return nil
		}
		rSig, err := w.vm.snowCtx.WarpSigner.Sign(msg)
		if err != nil {
			w.vm.snowCtx.Log.Warn("could not sign outgoing warp message", zap.Error(err))
			return nil
		}
		if err := w.vm.StoreWarpSignature(txID, w.vm.snowCtx.PublicKey, rSig); err != nil {
			w.vm.snowCtx.Log.Warn("could not store warp signature", zap.Error(err))
			return nil
		}
		sig = &chain.WarpSignature{
			PublicKey: w.vm.pkBytes,
			Signature: rSig,
		}
	}
	wp := codec.NewWriter(maxWarpResponse)
	wp.PackFixedBytes(sig.PublicKey)
	wp.PackFixedBytes(sig.Signature)
	if err := wp.Err(); err != nil {
		w.vm.snowCtx.Log.Warn("could not encode warp signature", zap.Error(err))
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
	publicKey := make([]byte, bls.PublicKeyLen)
	r.UnpackFixedBytes(bls.PublicKeyLen, &publicKey)
	signature := make([]byte, bls.SignatureLen)
	r.UnpackFixedBytes(bls.SignatureLen, &signature)
	if err := r.Err(); err != nil {
		w.vm.snowCtx.Log.Warn("could not decode warp signature", zap.Error(err))
		return nil
	}

	// Check public key is expected
	if !bytes.Equal(publicKey, job.publicKey) {
		w.vm.snowCtx.Log.Warn(
			"public key mismatch",
			zap.String("found", hex.EncodeToString(publicKey)),
			zap.String("expected", hex.EncodeToString(job.publicKey)),
		)
		return nil
	}

	// Check signature validity
	pk, err := bls.PublicKeyFromBytes(publicKey)
	if err != nil {
		w.vm.snowCtx.Log.Warn("could not decode public key", zap.Error(err))
		return nil
	}
	sig, err := bls.SignatureFromBytes(signature)
	if err != nil {
		w.vm.snowCtx.Log.Warn("could not decode signature", zap.Error(err))
		return nil
	}
	if !bls.Verify(pk, sig, job.msg) {
		w.vm.snowCtx.Log.Warn("could not verify signature")
		return nil
	}

	// Store in DB
	if err := w.vm.StoreWarpSignature(job.txID, pk, signature); err != nil {
		w.vm.snowCtx.Log.Warn("could not store warp signature", zap.Error(err))
		return nil
	}

	w.vm.snowCtx.Log.Info(
		"fetched and stored signature",
		zap.Stringer("txID", job.txID),
		zap.Stringer(
			"nodeID",
			job.nodeID,
		),
		zap.String("publicKey", hex.EncodeToString(job.publicKey)),
	)
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

	// Drop if we've already retried too many times
	if job.retry >= maxRetries {
		w.vm.snowCtx.Log.Info(
			"fetch job failed",
			zap.Stringer("nodeID", job.nodeID),
			zap.Stringer("txID", job.txID),
		)
		return nil
	}
	job.retry++

	w.l.Lock()
	w.pendingJobs.Push(&heap.Entry[*signatureJob, int64]{
		ID:    job.id,
		Item:  job,
		Val:   time.Now().Unix() + int64(backoffIncrease*job.retry),
		Index: w.pendingJobs.Len(),
	})
	w.l.Unlock()
	return nil
}

func (w *WarpManager) Done() {
	<-w.done
}
