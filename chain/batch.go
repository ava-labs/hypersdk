package chain

import (
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/codec"
)

type Batch struct{}

func NewBatch(authRegistry *codec.TypeParser[Auth, *warp.Message, bool], authTypes map[uint8]int) *Batch {
	for t, count := range authTypes {
		// Already verified that all authTypes are valid
		a, _, _ := authRegistry.LookupIndex(t)
		a.
	}
}

func (b *Batch) Add(digest []byte, auth Auth) {
}

func (b *Batch) Verify() {
	// Should interleave everything on a single sigjob, but should allocate sub-batches based on cores
	// This would mean batch would need to put on sigverify? unless just put directly if no batch?
}
