package chain

import (
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/workers"
)

type BatchVerifier interface {
	Add([]byte, Auth)
	Done() []func() error
}

type Batch struct {
	job *workers.Job
	bvs map[uint8]BatchVerifier
}

func NewBatch(job *workers.Job, authRegistry *codec.TypeParser[Auth, *warp.Message, bool], authTypes map[uint8]int) *Batch {
	bvs := map[uint8]BatchVerifier{}
	for t, count := range authTypes {
		// Already verified that all authTypes are valid
		a, _, _ := authRegistry.LookupIndex(t)
		bv, ok := a.GetBatchVerifier(job.Workers(), count)
		if !ok {
			continue
		}
		bvs[t] = bv
	}
	return &Batch{job, bvs}
}

func (b *Batch) Add(digest []byte, auth Auth) {
	// if batch doesn't exist for auth, just add write to job and start
	bv, ok := bvs[auth.GetTypeID()]
	if !ok {
		b.job.Go(auth.AsyncVerify(digest))
		return
	}
	bv.Add(digest, auth)
}

func (b *Batch) Done() {
	// Should interleave everything on a single sigjob, but should allocate sub-batches based on cores
	// This would mean batch would need to put on sigverify? unless just put directly if no batch?
	for _, bv := range b.bvs {
		for _, item := range bv.Done() {
			b.Job.Go(item)
		}
	}
}
