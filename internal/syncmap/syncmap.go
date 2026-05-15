// Package syncmap provides a generic, type-safe wrapper around sync.Map.
package syncmap

import "sync"

// Map is the interface for a type-safe concurrent map safe for use by multiple
// goroutines without additional locking.
// Obtain an implementation via New.
type Map[K comparable, V any] interface {
	// Load returns the value stored for key, or zero value and false if absent.
	Load(K) (V, bool)
	// Store sets the value for key.
	Store(K, V)
	// LoadOrStore returns the existing value for key when present; otherwise
	// stores and returns val. loaded is true when the value was loaded.
	LoadOrStore(K, V) (actual V, loaded bool)
	// Delete deletes the value for key.
	Delete(K)
	// Range calls f for each key–value pair in an unspecified order.
	// If f returns false, Range stops.
	Range(func(K, V) bool)
}

// Option is a functional option for Map configuration.
type Option[K comparable, V any] func(*mapImpl[K, V])

// mapImpl is the private implementation of Map[K,V].
type mapImpl[K comparable, V any] struct {
	m sync.Map
}

// New returns a ready-to-use Map[K,V].
func New[K comparable, V any](opts ...Option[K, V]) Map[K, V] {
	mm := &mapImpl[K, V]{}
	for _, o := range opts {
		o(mm)
	}
	return mm
}

func (mm *mapImpl[K, V]) Load(key K) (V, bool) {
	v, ok := mm.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	return v.(V), true
}

func (mm *mapImpl[K, V]) Store(key K, val V) { mm.m.Store(key, val) }

func (mm *mapImpl[K, V]) LoadOrStore(key K, val V) (actual V, loaded bool) {
	a, l := mm.m.LoadOrStore(key, val)
	return a.(V), l
}

func (mm *mapImpl[K, V]) Delete(key K) { mm.m.Delete(key) }

func (mm *mapImpl[K, V]) Range(f func(K, V) bool) {
	mm.m.Range(func(k, v any) bool { return f(k.(K), v.(V)) })
}
