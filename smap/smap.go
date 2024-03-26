package smap

import (
	"sync"

	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/zeebo/xxh3"
)

type SMap[V any] struct {
	count  uint64 // less coversions with [xxh3.HashString]
	shards []*shard[V]
}

func New[V any](shards int, shardSize int) *SMap[V] {
	m := &SMap[V]{
		count:  uint64(shards),
		shards: make([]*shard[V], shards),
	}
	for i := 0; i < shards; i++ {
		m.shards[i] = &shard[V]{data: make(map[string]V, shardSize)}
	}
	return m
}

type shard[V any] struct {
	l    sync.RWMutex
	data map[string]V
}

func (m *SMap[V]) Set(key string, value V) {
	h := xxh3.HashString(key)
	shard := m.shards[h%m.count]

	shard.l.Lock()
	defer shard.l.Unlock()
	shard.data[key] = value
}

func (m *SMap[V]) Get(key string) (V, bool) {
	h := xxh3.HashString(key)
	shard := m.shards[h%m.count]

	shard.l.RLock()
	defer shard.l.RUnlock()
	value, ok := shard.data[key]
	return value, ok
}

func (m *SMap[V]) Delete(key string) {
	h := xxh3.HashString(key)
	shard := m.shards[h%m.count]

	shard.l.Lock()
	defer shard.l.Unlock()
	delete(shard.data, key)
}

func (m *SMap[V]) Len() uint64 {
	var l uint64
	for _, shard := range m.shards {
		shard.l.RLock()
		l += uint64(len(shard.data))
		shard.l.RUnlock()
	}
	return l
}

func (m *SMap[V]) Merge(other *SMap[maybe.Maybe[any]]) error {
	if m.count != other.count {
		return ErrDifferentShardCount
	}
	g := &sync.WaitGroup{}
	for i := 0; i < int(m.count); i++ {
		g.Add(1)
		go func(i int) {
			defer g.Done()

			shard := m.shards[i]
			oshard := other.shards[i]
			shard.l.Lock()
			for k, v := range oshard.data {
				if v.IsNothing() {
					delete(shard.data, k)
				} else {
					shard.data[k] = v.Value().(V)
				}
			}
			shard.l.Unlock()
		}(i)
	}
	g.Wait()
	return nil
}
