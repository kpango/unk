package model

// section_cache.go — section cache key, line-count estimation, and grep matching.

import (
	"strconv"

	"github.com/kpango/unk/internal/siftlib"
	"github.com/kpango/unk/internal/tui/patch"
	"github.com/kpango/unk/internal/tui/textutil"
	"github.com/kpango/unk/internal/types"
)

// isASCII reports whether b contains only bytes in the 0x00–0x7F range.
// Used to select siftlib.SearchLinesCI (BytesToLower path) vs FindMatches ((?i) path).
func isASCII(b []byte) bool {
	for _, c := range b {
		if c > 0x7f {
			return false
		}
	}
	return true
}

// --- grep matching ---

// computeGrepMatches scans the rendered section cache to build a sorted list of
// absolute diff-line numbers that contain the current grepQuery. Call this after
// prewarm completes (sections are fully rendered) or immediately when grepQuery
// changes if sections are already cached.
//
// It also populates grepMatchFileSet: the set of rawVisibleFiles indices that
// have at least one content match. Files not yet prewarmed (no section cache
// entry) are conservatively included so they remain visible until their section
// is rendered and re-evaluated. grepMatchLines offsets are computed against the
// grep-filtered file order (matching visibleFiles), keeping n/N navigation
// consistent with the diff pane.
//
// Uses index scanning instead of strings.Split to avoid []string allocation.
func (m *model) computeGrepMatches() {
	if m.grepQuery == "" {
		m.grepMatchLines = nil
		m.grepMatchIdx = 0
		m.grepMatchFileSet = nil
		m.grepRegex = nil
		m.markSidebarDirty()
		m.sidebarEntriesDirty = true
		return
	}

	// Compile grepQuery using siftlib.CompilePattern — sift's (?i) approach with
	// literal fallback for invalid patterns. Stored on the model so the highlight
	// pipeline (OverlayGrepHighlight) can reuse the same compiled regex.
	re := siftlib.CompilePattern(m.grepQuery)
	m.grepRegex = re

	type fileScan struct {
		matchLines []int
		lineCount  int
	}
	rawFiles := m.rawVisibleFiles()
	scans := make([]fileScan, len(rawFiles))
	matchFileSet := make(map[int]bool, len(rawFiles))

	for i, f := range rawFiles {
		key := m.sectionCacheKey(f)
		section, ok := m.sectionCache[key]
		if !ok {
			// Section not yet prewarmed; include conservatively.
			scans[i] = fileScan{lineCount: m.sectionLineCountEstimate(f)}
			matchFileSet[i] = true
			continue
		}

		// Strip ANSI escapes from the whole section so the regex runs against
		// readable text rather than escape sequences.
		plain := []byte(textutil.StripANSI(section))

		// Use siftlib.FindMatches — adapted from sift's getMatches+countLines —
		// to get Match values with per-match line numbers and line text in one pass.
		// For purely ASCII diff content, siftlib.SearchLinesCI (BytesToLower path)
		// is used instead so sift's SIMD bytesToLower runs on the buffer.
		var matchLines []int
		if isASCII(plain) {
			matchLines = siftlib.SearchLinesCI(plain, m.grepQuery)
		} else {
			matchLines = siftlib.LineNumbers(siftlib.FindMatches(plain, nil, re))
		}

		lc := siftlib.CountNewlines([]byte(section)) + 1
		if len(section) == 0 {
			lc = 0
		}
		scans[i] = fileScan{matchLines: matchLines, lineCount: lc}
		if len(matchLines) > 0 {
			matchFileSet[i] = true
		}
	}

	m.grepMatchFileSet = matchFileSet

	// Compute absolute line numbers relative to the grep-filtered diff pane.
	// Only files in matchFileSet contribute to the rendered pane, so offsets
	// must skip non-matching files (they are hidden from the diff pane).
	var matches []int
	offset := 0
	for i, sc := range scans {
		if !matchFileSet[i] {
			continue
		}
		for _, lineIdx := range sc.matchLines {
			matches = append(matches, offset+lineIdx)
		}
		offset += sc.lineCount
	}

	m.grepMatchLines = matches
	if m.grepMatchIdx >= len(matches) {
		m.grepMatchIdx = 0
	}

	// Clamp selectedFileIndex to the new (potentially smaller) visible file list.
	n := len(m.visibleFiles())
	if m.selectedFileIndex >= n {
		m.selectedFileIndex = max(0, n-1)
		m.selectedUnkIndex = 0
	}
	m.markSidebarDirty()
	m.sidebarEntriesDirty = true
}

// --- section line-count estimation ---

// sectionLineCountEstimate returns the expected number of terminal lines for f
// in the current layout.
//
// All three layout modes differ from fileSectionLineCount (which counts patch
// elements 1-to-1):
//
// Unified: every patch element → 1 line. Estimate = fileSectionLineCount. ✓
//
// Split: del/add blocks are PAIRED — N dels + M adds → max(N,M) rows instead of
// N+M elements. Savings = sum(min(dels_i, adds_i)) across blocks.
// split_actual = fileSectionLineCount - splitSavings(patch)
//
// Stack: empty elements are SKIPPED (−1 per empty) and a separator is INSERTED
// for balanced unks (+1 per balanced unk). showUnkHeaders=false removes @@ lines.
// stack_actual = fileSectionLineCount - emptyCount + separators [- nUnks if !showHH]
func (m *model) sectionLineCountEstimate(f types.DiffFile) int {
	if f.IsBinary || f.IsTooLarge {
		return 2
	}
	switch m.layout.LayoutMode {
	case types.LayoutModeStack:
		return m.sectionLineCountStack(f)
	case types.LayoutModeSplit:
		key := f.Metadata.CacheKey
		if key == "" {
			key = f.ID
		}
		savings, ok := m.splitSavingsCache[key]
		if !ok {
			savings = patch.SplitViewLineSavings(f.Patch)
			if m.splitSavingsCache == nil {
				m.splitSavingsCache = make(map[string]int, 16)
			}
			m.splitSavingsCache[key] = savings
		}
		n := fileSectionLineCount(f) - savings
		if !m.showUnkHeaders {
			n -= patch.CountUnks(f.Patch)
		}
		return n
	default: // LayoutModeUnified or any other non-split/stack mode
		return fileSectionLineCount(f)
	}
}

// sectionLineCountStack computes the section line count for stack layout mode.
func (m *model) sectionLineCountStack(f types.DiffFile) int {
	return fileSectionLineCount(f) + patch.StackViewLineAdjust(f.Patch, m.showUnkHeaders)
}

// --- cache key ---

// sectionCacheKey returns the lookup key for a file section in sectionCache.
// The key encodes file content (CacheKey/ID) and horizontal scroll offset.
// All other rendering state (theme, layout, flags) is handled by clearRenderCache.
func (m *model) sectionCacheKey(f types.DiffFile) string {
	ck := f.Metadata.CacheKey
	if ck == "" {
		ck = f.ID
	}
	if m.codeHorizontalOffset != 0 {
		ck += "|" + strconv.Itoa(m.codeHorizontalOffset)
	}
	if m.grepQuery != "" {
		ck += "|grep:" + m.grepQuery
	}
	return ck
}
