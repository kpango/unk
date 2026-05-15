package model

// section_cache.go — section cache key, line-count estimation, and grep matching.

import (
	"strconv"
	"strings"

	"github.com/kpango/unk/internal/tui/patch"
	"github.com/kpango/unk/internal/tui/textutil"
	"github.com/kpango/unk/internal/types"
)

// --- grep matching ---

// computeGrepMatches scans the rendered section cache to build a sorted list of
// absolute diff-line numbers that contain the current grepQuery. Call this after
// prewarm completes (sections are fully rendered) or immediately when grepQuery
// changes if sections are already cached. If some sections are not yet cached,
// their estimated line count is still added to offset so later sections remain
// at correct positions; those sections will produce matches on the next call.
// Uses index scanning instead of strings.Split to avoid []string allocation.
func (m *model) computeGrepMatches() {
	if m.grepQuery == "" {
		m.grepMatchLines = nil
		m.grepMatchIdx = 0
		return
	}
	query := strings.ToLower(m.grepQuery)
	var matches []int
	offset := 0
	for _, f := range m.visibleFiles() {
		key := m.sectionCacheKey(f)
		section, ok := m.sectionCache[key]
		if !ok {
			offset += m.sectionLineCountEstimate(f)
			continue
		}
		lineIdx := 0
		s := section
		for {
			nl := strings.IndexByte(s, '\n')
			var line string
			if nl < 0 {
				line = s
			} else {
				line = s[:nl]
			}
			plain := textutil.StripANSI(line)
			if strings.Contains(strings.ToLower(plain), query) {
				matches = append(matches, offset+lineIdx)
			}
			lineIdx++
			if nl < 0 {
				break
			}
			s = s[nl+1:]
		}
		offset += lineIdx
	}
	m.grepMatchLines = matches
	if m.grepMatchIdx >= len(matches) {
		m.grepMatchIdx = 0
	}
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
