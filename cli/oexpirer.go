package cli

import (
	"sync"

	"github.com/ava-labs/hypersdk/list"

	"github.com/ava-labs/avalanchego/ids"
)

type OExpirer struct {
	l sync.RWMutex
	q *list.List[*txWrapper]
	m map[ids.ID]*list.Element[*txWrapper]
}

func NewOExpirer() *OExpirer {
	return &OExpirer{
		q: &list.List[*txWrapper]{},
		m: make(map[ids.ID]*list.Element[*txWrapper], 16_384),
	}
}

func (o *OExpirer) Add(tx *txWrapper) {
	o.l.Lock()
	defer o.l.Unlock()

	e := o.q.PushBack(tx)
	o.m[tx.ID()] = e
}

func (o *OExpirer) Remove(id ids.ID) *txWrapper {
	o.l.Lock()
	defer o.l.Unlock()

	e, ok := o.m[id]
	if !ok {
		return nil
	}

	o.q.Remove(e)
	delete(o.m, id)
	return e.Value()
}

func (o *OExpirer) SetMin(t int64) []*txWrapper {
	o.l.Lock()
	defer o.l.Unlock()

	expired := []*txWrapper{}
	for {
		e := o.q.First()
		if e == nil {
			break
		}
		if e.Value().Expiry() > t {
			break
		}
		o.q.Remove(e)
		delete(o.m, e.Value().ID())
		expired = append(expired, e.Value())
	}
	return expired
}

func (o *OExpirer) Len() int {
	o.l.RLock()
	defer o.l.RUnlock()

	return len(o.m)
}
