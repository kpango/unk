// Package concurrent provides generic utilities for concurrent operations.
package concurrent

import (
	"runtime"
	"sync"
)

// Result holds a value or error from a concurrent operation.
type Result[T any] struct {
	Value T
	Err   error
}

// Map applies fn to each element concurrently using a bounded goroutine pool
// (min(NumCPU, len(items)) workers), preserving order.
func Map[T, U any](items []T, fn func(T) (U, error)) []Result[U] {
	n := len(items)
	results := make([]Result[U], n)
	if n == 0 {
		return results
	}
	workers := min(runtime.NumCPU(), n)
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		go func(i int, item T) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			v, err := fn(item)
			results[i] = Result[U]{Value: v, Err: err}
		}(i, item)
	}
	wg.Wait()
	return results
}
