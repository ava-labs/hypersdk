package smap

import (
	"runtime"
	"sync"

	"github.com/zeebo/xxh3"
)

var shardCount = runtime.NumCPU() * 16

type SMap[V any] struct {
	count  uint64 // less coversions with [xxh3.HashString]
	shards []*shard[V]
}

func New[V any](initial int) *SMap[V] {
	m := &SMap[V]{
		count:  uint64(shardCount),
		shards: make([]*shard[V], shardCount),
	}
	for i := 0; i < shardCount; i++ {
		m.shards[i] = &shard[V]{data: make(map[string]V, max(16, initial/shardCount))}
	}
	return m
}

type shard[V any] struct {
	l    sync.RWMutex
	data map[string]V
}

func (m *SMap[V]) Put(key string, value V) {
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

func (m *SMap[V]) GetAndDelete(key string) (V, bool) {
	h := xxh3.HashString(key)
	shard := m.shards[h%m.count]

	shard.l.Lock()
	defer shard.l.Unlock()
	value, ok := shard.data[key]
	if ok {
		delete(shard.data, key)
	}
	return value, ok
}

func (m *SMap[V]) Delete(key string) {
	h := xxh3.HashString(key)
	shard := m.shards[h%m.count]

	shard.l.Lock()
	defer shard.l.Unlock()
	delete(shard.data, key)
}

func (m *SMap[V]) Iterate(f func(k string, v V) bool) {
	for i := 0; i < int(m.count); i++ {
		shard := m.shards[i]
		shard.l.RLock()
		for k, v := range shard.data {
			if !f(k, v) {
				shard.l.RUnlock()
				return
			}
		}
		shard.l.RUnlock()
	}
}
