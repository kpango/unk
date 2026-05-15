package textutil

import (
	"bytes"

	"github.com/kpango/unk/internal/pool"
)

// --- builder pool ---

var builderPool = pool.New(func() *bytes.Buffer { return new(bytes.Buffer) })

// AcquireBuilder returns a reset *bytes.Buffer from the pool.
func AcquireBuilder() *bytes.Buffer {
	b := builderPool.Get()
	b.Reset()
	return b
}

// ReleaseBuilder returns b to the pool. Buffers larger than 1 MiB are discarded.
func ReleaseBuilder(b *bytes.Buffer) {
	if b.Cap() <= 1<<20 {
		builderPool.Put(b)
	}
}

// --- patch-lines pool ---

var patchLinesPool = pool.New(func() *[]string { sl := make([]string, 0, 256); return &sl })

// AcquirePatchLines splits patch on newlines into a pooled []string and returns
// both the pool handle (for release) and the populated slice.
func AcquirePatchLines(patch string) (*[]string, []string) {
	sp := patchLinesPool.Get()
	lines := SplitLines(patch, (*sp)[:0])
	*sp = lines
	return sp, lines
}

// ReleasePatchLines returns sp to the pool.
func ReleasePatchLines(sp *[]string) {
	if cap(*sp) <= 4096 {
		patchLinesPool.Put(sp)
	}
}
