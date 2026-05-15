package render

import (
	"strings"
	"sync"

	"github.com/kpango/unk/internal/diff"
	"github.com/kpango/unk/internal/tui/textutil"
	"github.com/kpango/unk/internal/types"
)

// IntraBatchSize is the optimal goroutine count for BuildIntraCache.
// Empirically determined: N=16 minimises wall time on 50-file changesets by
// balancing goroutine spawn overhead (~8µs each) against per-goroutine
// computation time.
const IntraBatchSize = 16

// intraBatchSize returns the target goroutine count for BuildIntraCache.
func intraBatchSize(nFiles int) int {
	return min(nFiles, IntraBatchSize)
}

// BuildIntraCache pre-computes intra-line diff spans for all files in a changeset
// using a bounded goroutine pool (min(intraBatchSize, nFiles) workers).
// Stored as [2][][]diff.IntraSpan per file CacheKey: index 0=del, 1=add spans.
// The inner slice is indexed directly by patch-line position (dense array, O(1) access).
//
// Each goroutine accumulates all spans for its chunk in growing delBuf/addBuf slices
// without resetting between files. Sub-slices stored in results point to non-overlapping
// regions of these backing arrays, so they remain valid as the buffers grow.
func BuildIntraCache(cache map[string][2][][]diff.IntraSpan, files []types.DiffFile) {
	work := make([]types.DiffFile, 0, len(files))
	keys := make([]string, 0, len(files))
	for _, f := range files {
		if f.IsBinary || f.IsTooLarge || f.Patch == "" {
			continue
		}
		key := f.Metadata.CacheKey
		if key == "" {
			key = f.ID
		}
		if _, exists := cache[key]; exists {
			continue
		}
		work = append(work, f)
		keys = append(keys, key)
	}
	n := len(work)
	if n == 0 {
		return
	}

	type entry struct {
		key  string
		data [2][][]diff.IntraSpan
	}

	nWorkers := intraBatchSize(n)
	chunkSize := (n + nWorkers - 1) / nWorkers
	nActual := (n + chunkSize - 1) / chunkSize
	results := make([]entry, n)

	chunkLineCounts := make([]int, nActual)
	for w := range nActual {
		start := w * chunkSize
		end := min(start+chunkSize, n)
		total := 0
		for _, f := range work[start:end] {
			total += strings.Count(f.Patch, "\n") + 1
		}
		chunkLineCounts[w] = total
	}

	var wg sync.WaitGroup
	wg.Add(nActual)
	for w := range nActual {
		start := w * chunkSize
		end := min(start+chunkSize, n)
		go func(fs []types.DiffFile, ks []string, base int, totalLines int) {
			defer wg.Done()
			ws := diff.AcquireWS()
			defer diff.ReleaseWS(ws)
			patchLines := make([]string, 0, 512)
			delBuf := make([]diff.IntraSpan, 0, 256)
			addBuf := make([]diff.IntraSpan, 0, 256)
			flatDel := make([][]diff.IntraSpan, totalLines)
			flatAdd := make([][]diff.IntraSpan, totalLines)
			lineBase := 0
			for i, f := range fs {
				patchLines = textutil.SplitLines(f.Patch, patchLines[:0])
				nLines := len(patchLines)
				delSpans := flatDel[lineBase : lineBase+nLines]
				addSpans := flatAdd[lineBase : lineBase+nLines]
				lineBase += nLines
				inUnk := false
				for j := range nLines {
					if len(patchLines[j]) == 0 {
						continue
					}
					if patchLines[j][0] == '@' {
						inUnk = true
						continue
					}
					if !inUnk || patchLines[j][0] != '-' {
						continue
					}
					addIdx := j + 1
					if addIdx >= nLines || len(patchLines[addIdx]) == 0 || patchLines[addIdx][0] != '+' {
						continue
					}
					dStart, aStart := len(delBuf), len(addBuf)
					delBuf, addBuf = diff.IntraLineDiffInto(ws, delBuf, addBuf, patchLines[j][1:], patchLines[addIdx][1:])
					if len(delBuf) > dStart {
						delSpans[j] = delBuf[dStart:len(delBuf):len(delBuf)]
						addSpans[addIdx] = addBuf[aStart:len(addBuf):len(addBuf)]
					}
				}
				results[base+i] = entry{ks[i], [2][][]diff.IntraSpan{delSpans, addSpans}}
				delBuf = delBuf[:0]
				addBuf = addBuf[:0]
			}
		}(work[start:end], keys[start:end], start, chunkLineCounts[w])
	}
	wg.Wait()

	for _, r := range results {
		cache[r.key] = r.data
	}
}
