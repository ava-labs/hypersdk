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
	if !ok {
		hl = &holderLock{}
		l.m[key] = hl
	}
	hl.holders++
	l.l.Unlock()

	// Another caller may grab this lock before we do, however,
	// that's fine.
	if write {
		hl.mu.Lock()
	} else {
		hl.mu.RLock()
	}
}

func (l *Lockmap) unlock(key string, write bool) {
	l.l.Lock()
	defer l.l.Unlock()

	hl := l.m[key]

	// While unlocking these mutexes isn't required
	// if we are the last holder, we still do it as
	// a best practice.
	if write {
		hl.mu.Unlock()
	} else {
		hl.mu.RUnlock()
	}

	if hl.holders == 1 {
		delete(l.m, key)
	} else {
		hl.holders--
	}
}

func (l *Lockmap) Locks() int {
	l.l.Lock()
	defer l.l.Unlock()

	return len(l.m)
}
