package xmap

import (
	"sync"
)

// Map is a generic type-safe wrapper around sync.Map.
type Map[K comparable, V any] struct {
	m sync.Map
}

// New creates a new Map instance.
func New[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{}
}

// Load returns the value stored in the map for a key, or the zero value if no
// value is present. The ok result indicates whether value was found in the map.
func (m *Map[K, V]) Load(key K) (value V, ok bool) {
	v, ok := m.m.Load(key)
	if !ok {
		return value, false
	}

	//nolint:forcetypeassert // Safe to assert since we control the map.
	return v.(V), true
}

// Store sets the value for a key.
func (m *Map[K, V]) Store(key K, value V) {
	m.m.Store(key, value)
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (m *Map[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	v, loaded := m.m.LoadOrStore(key, value)
	//nolint:forcetypeassert // Safe to assert since we control the map.
	return v.(V), loaded
}

// LoadAndDelete deletes the value for a key, returning the previous value if any.
// The loaded result reports whether the key was present.
func (m *Map[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	v, loaded := m.m.LoadAndDelete(key)
	if !loaded {
		return value, false
	}

	//nolint:forcetypeassert // Safe to assert since we control the map.
	return v.(V), true
}

// Delete deletes the value for a key.
func (m *Map[K, V]) Delete(key K) {
	m.m.Delete(key)
}

// Swap swaps the value for a key and returns the previous value if any.
// The loaded result reports whether the key was present.
func (m *Map[K, V]) Swap(key K, value V) (previous V, loaded bool) {
	v, loaded := m.m.Swap(key, value)
	if !loaded {
		return previous, false
	}

	//nolint:forcetypeassert // Safe to assert since we control the map.
	return v.(V), true
}

// CompareAndSwap swaps the old and new values for key
// if the value stored in the map is equal to old.
//
//nolint:predeclared // Temporary variable name.
func (m *Map[K, V]) CompareAndSwap(key K, old, new V) (swapped bool) {
	return m.m.CompareAndSwap(key, old, new)
}

// CompareAndDelete deletes the entry for key if its value is equal to old.
func (m *Map[K, V]) CompareAndDelete(key K, old V) (deleted bool) {
	return m.m.CompareAndDelete(key, old)
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	m.m.Range(func(key, value any) bool {
		//nolint:forcetypeassert // Safe to assert since we control the map.
		return f(key.(K), value.(V))
	})
}

// Clear deletes all entries from the map.
func (m *Map[K, V]) Clear() {
	m.m.Clear()
}
