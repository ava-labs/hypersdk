package lockmap

import "sync"

type holderLock struct {
	holders int
	mu      sync.RWMutex
}

type Lockmap struct {
	l sync.Mutex
	m map[string]*holderLock
}

func New(initSize int) *Lockmap {
	return &Lockmap{
		m: make(map[string]*holderLock, initSize),
	}
}

func (l *Lockmap) Lock(key string) {
	l.lock(key, true)
}

func (l *Lockmap) Unlock(key string) {
	l.unlock(key, true)
}

func (l *Lockmap) RLock(key string) {
	l.lock(key, false)
}

func (l *Lockmap) RUnlock(key string) {
	l.unlock(key, false)
}

func (l *Lockmap) lock(key string, write bool) {
	l.l.Lock()
	hl, ok := l.m[key]
	if ok {
		hl.holders++
		l.l.Unlock()

		if write {
			hl.mu.Lock()
		} else {
			hl.mu.RLock()
		}
		return
	}
	hl = &holderLock{holders: 1}
	if write {
		hl.mu.Lock()
	} else {
		hl.mu.RLock()
	}
	l.m[key] = hl
	l.l.Unlock()
}

func (l *Lockmap) unlock(key string, write bool) {
	l.l.Lock()
	hl := l.m[key]
	if hl.holders > 1 {
		hl.holders--
		l.l.Unlock()
		if write {
			hl.mu.Unlock()
		} else {
			hl.mu.RUnlock()
		}
	} else {
		delete(l.m, key)
	}
	l.l.Unlock()
}

func (l *Lockmap) Locks() int {
	l.l.Lock()
	defer l.l.Unlock()

	return len(l.m)
}
