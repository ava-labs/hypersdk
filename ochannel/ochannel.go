package ochannel

import "sync"

type OChannel[T interface{}] struct {
	l     sync.Mutex
	count int
	m     map[int]T
	c     chan T
}

func New[T interface{}](c chan T) *OChannel[T] {
	return &OChannel[T]{
		m: make(map[int]T),
		c: c,
	}
}

func (o *OChannel[T]) Add(t T, i int) {
	o.l.Lock()
	defer o.l.Unlock()

	// Stage for later
	if i != o.count {
		o.m[i] = t
		return
	}

	// Send this item
	o.count++
	o.c <- t

	// Process backlog
	for {
		if t, ok := o.m[o.count]; ok {
			delete(o.m, o.count)
			o.count++
			o.c <- t
		} else {
			break
		}
	}
}
