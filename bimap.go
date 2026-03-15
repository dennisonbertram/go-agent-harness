package main

// BiMap is a bidirectional map supporting fast O(1) lookups in both directions.
type BiMap[K comparable, V comparable] struct {
	keyToValue map[K]V
	valueToKey map[V]K
}

// NewBiMap creates a new BiMap.
func NewBiMap[K comparable, V comparable]() *BiMap[K, V] {
	return &BiMap[K, V]{
		keyToValue: make(map[K]V),
		valueToKey: make(map[V]K),
	}
}

// Put inserts (k, v). If k or v already exists, it replaces the old mapping.
func (m *BiMap[K, V]) Put(k K, v V) {
	if ov, ok := m.keyToValue[k]; ok {
		delete(m.valueToKey, ov)
	}
	if okk, ok := m.valueToKey[v]; ok {
		delete(m.keyToValue, okk)
	}
	m.keyToValue[k] = v
	m.valueToKey[v] = k
}

// GetByKey looks up by key.
func (m *BiMap[K, V]) GetByKey(k K) (V, bool) {
	v, ok := m.keyToValue[k]
	return v, ok
}

// GetByValue looks up by value.
func (m *BiMap[K, V]) GetByValue(v V) (K, bool) {
	k, ok := m.valueToKey[v]
	return k, ok
}

// DeleteByKey removes entry by key.
func (m *BiMap[K, V]) DeleteByKey(k K) {
	if v, ok := m.keyToValue[k]; ok {
		delete(m.keyToValue, k)
		delete(m.valueToKey, v)
	}
}

// DeleteByValue removes entry by value.
func (m *BiMap[K, V]) DeleteByValue(v V) {
	if k, ok := m.valueToKey[v]; ok {
		delete(m.valueToKey, v)
		delete(m.keyToValue, k)
	}
}

// Len returns number of elements.
func (m *BiMap[K, V]) Len() int {
	return len(m.keyToValue)
}
