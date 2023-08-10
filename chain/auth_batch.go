package chain

import (
	"github.com/ava-labs/hypersdk/workers"
)

// TODO: change name to AuthBatch
type Batch struct {
	job *workers.Job
	bvs map[uint8]AuthBatchAsyncVerifier
}

func NewBatch(vm VM, job *workers.Job, authTypes map[uint8]int) *Batch {
	bvs := map[uint8]AuthBatchAsyncVerifier{}
	for t, count := range authTypes {
		// TODO: move this to a registry?
		bv, ok := vm.GetBatchAsyncVerifier(t, job.Workers(), count)
		if !ok {
			continue
		}
		bvs[t] = bv
	}
	return &Batch{job, bvs}
}

func (b *Batch) Add(digest []byte, auth Auth) {
	// if batch doesn't exist for auth, just add right to job and start
	bv, ok := b.bvs[auth.GetTypeID()]
	if !ok {
		b.job.Go(func() error { return auth.AsyncVerify(digest) })
		return
	}
	// May finish parts of batch early, let's start computing them as soon as possible
	if j := bv.Add(digest, auth); j != nil {
		b.job.Go(j)
	}
}

func (b *Batch) Done() {
	for _, bv := range b.bvs {
		// TODO: start bv's earlier?
		for _, item := range bv.Done() {
			b.job.Go(item)
		}
	}
}
