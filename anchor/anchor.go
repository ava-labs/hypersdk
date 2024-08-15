package anchor

import (
	"sync"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/bls"
)

type Anchor struct {
	vm   VM
	Url  string `json:"url"`
	UrlL sync.Mutex
}

func NewAnchor(url string, vm VM) *Anchor {
	return &Anchor{
		Url: url,
		vm:  vm,
	}
}

// only the Anchor provide the signature signed by the same key the validator has will be accepted
func (a *Anchor) Replace(url string) {
	a.UrlL.Lock()
	defer a.UrlL.Unlock()

	a.Url = url
}

func (a *Anchor) RequestAnchorDigest() ([]byte, error) {
	return nil, nil
}

// returns Slot, Txs, FeeReceiver, error
func (a *Anchor) RequestAnchorChunk(sig *bls.Signature) (*chain.Anchor, int64, []*chain.Transaction, codec.Address, error) {
	return nil, 0, nil, codec.EmptyAddress, nil
}
