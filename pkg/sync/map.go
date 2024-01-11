package sync

import "sync"

type TypedSyncMap[K comparable, V any] struct {
	m sync.Map
}

func (m *TypedSyncMap[K, V]) Delete(key K) { m.m.Delete(key) }

func (m *TypedSyncMap[K, V]) Load(key K) (value V, ok bool) {
	v, ok := m.m.Load(key)
	if !ok {
		return value, ok
	}
	return v.(V), ok
}

func (m *TypedSyncMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	v, loaded := m.m.LoadAndDelete(key)
	if !loaded {
		return value, loaded
	}
	return v.(V), loaded
}

func (m *TypedSyncMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	a, loaded := m.m.LoadOrStore(key, value)
	return a.(V), loaded
}

func (m *TypedSyncMap[K, V]) Store(key K, value V) { m.m.Store(key, value) }
