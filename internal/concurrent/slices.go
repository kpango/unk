// Synchronous slice helpers that complement the concurrent operations in concurrent.go.
package concurrent

// Transform applies fn to each element sequentially and returns the results.
// Use this for pure in-process transforms; use Map for I/O-bound or CPU-bound work.
func Transform[T, U any](items []T, fn func(T) U) []U {
	out := make([]U, len(items))
	for i, v := range items {
		out[i] = fn(v)
	}
	return out
}

// Collect returns only the successful values from a Map result.
func Collect[T any](results []Result[T]) []T {
	out := make([]T, 0, len(results))
	for _, r := range results {
		if r.Err == nil {
			out = append(out, r.Value)
		}
	}
	return out
}
