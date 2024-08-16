package anchor

import (
	"slices"
	"strings"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

type AnchorRegistry struct {
	clients  map[ids.ID]*Anchor
	clientsL sync.Mutex
	vm       VM
}

func NewAnchorRegistry(vm VM) *AnchorRegistry {
	return &AnchorRegistry{
		vm: vm,
	}
}

func (r *AnchorRegistry) Register(url string, namespace string) error {
	anchorCli := &Anchor{
		Url:       url,
		vm:        r.vm,
		Namespace: namespace,
	}
	clientID, err := anchorCli.ID()
	if err != nil {
		return err
	}

	r.clients[clientID] = anchorCli
	return nil
}

func (r *AnchorRegistry) Remove(url string) error {
	r.clientsL.Lock()
	defer r.clientsL.Unlock()

	cilentID, err := ids.ToID([]byte(url))
	if err != nil {
		return err
	}

	delete(r.clients, cilentID)
	return nil
}

func (r *AnchorRegistry) Len() int {
	return len(r.clients)
}

func (r *AnchorRegistry) SortedAnchors() []*Anchor {
	anchors := make([]*Anchor, 0, len(r.clients))
	for _, anchor := range r.clients {
		anchors = append(anchors, anchor)
	}

	slices.SortFunc(anchors, func(a, b *Anchor) int {
		return strings.Compare(a.Url, b.Url)
	})

	return anchors
}

type Anchor struct {
	vm        VM
	Url       string `json:"url"`
	UrlL      sync.Mutex
	Namespace string `json:"namespace"`
}

func NewAnchor(url string, vm VM) *Anchor {
	return &Anchor{
		Url: url,
		vm:  vm,
	}
}

func (a *Anchor) ID() (ids.ID, error) {
	return ids.ToID([]byte(a.Url))
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
