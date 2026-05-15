// Package pool provides a type-safe generic wrapper around sync.Pool.
// It eliminates the .(T) type assertion that sync.Pool normally requires at
// every Get call site — the same problem the syncmap package solves for sync.Map.
package pool

import "sync"

// Pool is the interface for type-safe generic pooling of *T values.
// Obtain an implementation via New.
type Pool[T any] interface {
	// Get retrieves a *T from the pool, or allocates a new one.
	Get() *T
	// Put returns a *T to the pool for reuse. The caller must not retain any
	// reference to v after calling Put.
	Put(*T)
}

// Option is a functional option for pool configuration.
type Option[T any] func(*poolImpl[T])

// poolImpl is the private implementation of Pool[T].
// A poolImpl must not be copied after first use (same constraint as sync.Pool).
// Declare pool variables at package level with New and never assign them again.
type poolImpl[T any] struct {
	p sync.Pool
}

// New returns a Pool[T] whose constructor calls newFn when the pool is empty.
// newFn must return a non-nil pointer.
func New[T any](newFn func() *T, opts ...Option[T]) Pool[T] {
	p := &poolImpl[T]{p: sync.Pool{New: func() any { return newFn() }}}
	for _, o := range opts {
		o(p)
	}
	return p
}

func (p *poolImpl[T]) Get() *T  { return p.p.Get().(*T) }
func (p *poolImpl[T]) Put(v *T) { p.p.Put(v) }
