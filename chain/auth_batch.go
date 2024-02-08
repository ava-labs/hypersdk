// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/workers"
)

const authWorkerBacklog = 16_384

type AuthVM interface {
	Logger() logging.Logger
	GetAuthBatchVerifier(authTypeID uint8, cores int, count int) (AuthBatchVerifier, bool)
}

// Adding a signature to a verification batch
// may perform complex cryptographic operations. We should
// not block the caller when this happens and we should
// not require each batch package to re-implement this logic.
type AuthBatch struct {
	vm  AuthVM
	job workers.Job
	bvs map[uint8]*authBatchWorker
}

func NewAuthBatch(vm AuthVM, job workers.Job, authTypes map[uint8]int) *AuthBatch {
	bvs := map[uint8]*authBatchWorker{}
	for t, count := range authTypes {
		bv, ok := vm.GetAuthBatchVerifier(t, job.Workers(), count)
		if !ok {
			continue
		}
		bw := &authBatchWorker{
			vm,
			job,
			bv,
			make(chan *authBatchObject, authWorkerBacklog),
			make(chan struct{}),
		}
		go bw.start()
		bvs[t] = bw
	}
	return &AuthBatch{vm, job, bvs}
}

func (a *AuthBatch) Add(digest []byte, auth Auth) {
	// If batch doesn't exist for auth, just add verify right to job and start
	// processing.
	bv, ok := a.bvs[auth.GetTypeID()]
	if !ok {
		a.job.Go(func() error { return auth.Verify(context.TODO(), digest) })
		return
	}
	bv.items <- &authBatchObject{digest, auth}
}

func (a *AuthBatch) Done(f func()) {
	for _, bw := range a.bvs {
		close(bw.items)
		<-bw.done

		for _, item := range bw.bv.Done() {
			a.job.Go(item)
			a.vm.Logger().Debug("enqueued batch for processing during done")
		}
	}
	a.job.Done(f)
}

type authBatchObject struct {
	digest []byte
	auth   Auth
}

type authBatchWorker struct {
	vm    AuthVM
	job   workers.Job
	bv    AuthBatchVerifier
	items chan *authBatchObject
	done  chan struct{}
}

func (b *authBatchWorker) start() {
	defer close(b.done)

	for object := range b.items {
		if j := b.bv.Add(object.digest, object.auth); j != nil {
			// May finish parts of batch early, let's start computing them as soon as possible
			b.job.Go(j)
			b.vm.Logger().Debug("enqueued batch for processing during add")
		}
	}
}
