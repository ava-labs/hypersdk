package oexpirer

import (
	"sync"

	"github.com/ava-labs/hypersdk/list"

	"github.com/ava-labs/avalanchego/ids"
)

type Item interface {
	ID() ids.ID
	Expiry() int64
}

type OExpirer[T Item] struct {
	l sync.RWMutex
	q *list.List[T]
	m map[ids.ID]*list.Element[T]
}

func New[T Item](init int) *OExpirer[T] {
	return &OExpirer[T]{
		q: &list.List[T]{},
		m: make(map[ids.ID]*list.Element[T], init),
	}
}

func (o *OExpirer[T]) Add(i T, front bool) {
	o.l.Lock()
	defer o.l.Unlock()

	if _, ok := o.m[i.ID()]; ok {
		return
	}
	var e *list.Element[T]
	if front {
		e = o.q.PushFront(i)
	} else {
		e = o.q.PushBack(i)
	}
	o.m[i.ID()] = e
}

func (o *OExpirer[T]) Remove(id ids.ID) (T, bool) {
	o.l.Lock()
	defer o.l.Unlock()

	e, ok := o.m[id]
	if !ok {
		return *new(T), false
	}

	o.q.Remove(e)
	delete(o.m, id)
	return e.Value(), true
}

func (o *OExpirer[T]) RemoveNext() (T, bool) {
	o.l.Lock()
	defer o.l.Unlock()

	e := o.q.First()
	if e == nil {
		return *new(T), false
	}

	o.q.Remove(e)
	delete(o.m, e.ID())
	return e.Value(), true
}

func (o *OExpirer[T]) Has(id ids.ID) bool {
	o.l.RLock()
	defer o.l.RUnlock()

	_, ok := o.m[id]
	return ok
}

func (o *OExpirer[T]) SetMin(t int64) []T {
	o.l.Lock()
	defer o.l.Unlock()

	expired := []T{}
	for {
		e := o.q.First()
		if e == nil {
			break
		}
		if e.Value().Expiry() >= t {
			break
		}
		o.q.Remove(e)
		delete(o.m, e.Value().ID())
		expired = append(expired, e.Value())
	}
	return expired
}

func (o *OExpirer[T]) Len() int {
	o.l.RLock()
	defer o.l.RUnlock()

	return len(o.m)
}
