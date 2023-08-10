package chain

import (
	"fmt"

	"github.com/ava-labs/hypersdk/workers"
)

// TODO: change name to AuthBatch
type Batch struct {
	job *workers.Job
	bvs map[uint8]*batchWorker
}

type batchObject struct {
	digest []byte
	auth   Auth
}

// batch async verifiers may have slow calls, we should
// not ask them to implement async handling
type batchWorker struct {
	job   *workers.Job
	bv    AuthBatchAsyncVerifier
	items chan *batchObject
	done  chan struct{}
}

func (b *batchWorker) start() {
	defer close(b.done)

	for object := range b.items {
		if j := b.bv.Add(object.digest, object.auth); j != nil {
			b.job.Go(j)
			fmt.Println("enqueued batch for processing during add")
		}
	}
}

func NewBatch(vm VM, job *workers.Job, authTypes map[uint8]int) *Batch {
	bvs := map[uint8]*batchWorker{}
	for t, count := range authTypes {
		// TODO: move this to a registry?
		bv, ok := vm.GetBatchAsyncVerifier(t, job.Workers(), count)
		if !ok {
			continue
		}
		bw := &batchWorker{job, bv, make(chan *batchObject, 40_000), make(chan struct{})}
		go bw.start()
		bvs[t] = bw
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
	bv.items <- &batchObject{digest, auth}
}

func (b *Batch) Done(f func()) {
	for _, bw := range b.bvs {
		close(bw.items)
		<-bw.done

		for _, item := range bw.bv.Done() {
			b.job.Go(item)
			fmt.Println("enqueued batch for processing during done")
		}
	}
	b.job.Done(f)
}
