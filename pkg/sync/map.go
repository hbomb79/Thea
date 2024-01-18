package sync

import "sync"

type TypedSyncMap[K comparable, V any] struct {
	m sync.Map
}

func (m *TypedSyncMap[K, V]) Delete(key K) { m.m.Delete(key) }

func (m *TypedSyncMap[K, V]) Load(key K) (V, bool) {
	v, ok := m.m.Load(key)
	if !ok {
		return *new(V), ok
	}

	if vv, ok := v.(V); ok {
		return vv, true
	}
	return *new(V), false
}

func (m *TypedSyncMap[K, V]) LoadAndDelete(key K) (V, bool) {
	v, loaded := m.m.LoadAndDelete(key)
	if !loaded {
		return *new(V), loaded
	}

	if vv, ok := v.(V); ok {
		return vv, loaded
	}
	return *new(V), loaded
}

func (m *TypedSyncMap[K, V]) LoadOrStore(key K, value V) (V, bool) {
	a, loaded := m.m.LoadOrStore(key, value)
	if av, ok := a.(V); ok {
		return av, loaded
	}

	return *new(V), loaded
}

func (m *TypedSyncMap[K, V]) Store(key K, value V) { m.m.Store(key, value) }
