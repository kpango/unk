package model

import (
	"strings"
	"unsafe"

	"github.com/kpango/unk/internal/tui/overlay"
	"github.com/kpango/unk/internal/tui/textutil"
	"github.com/kpango/unk/internal/types"
)

// fileView holds precomputed per-file section data for renderDiffPane.
// Built once per dirty cycle to avoid repeated sectionCacheKey + map lookups.
type fileView struct {
	file      types.DiffFile
	rendered  string    // section content (empty if not yet cached)
	offsets   []int32   // byte-start positions per line; nil if not cached
	lineCount int       // total lines in this section
	hasCached bool      // true if rendered is valid
}

// rebuildFileViewCache precomputes fileViewCache and fileViewStarts from the current
// sectionCache / sectionLineCache / sectionLineOffsets maps.
// Called lazily at the start of renderDiffPane when fileViewDirty is true.
func (m *model) rebuildFileViewCache() {
	files := m.visibleFiles()
	n := len(files)
	if cap(m.fileViewCache) >= n {
		m.fileViewCache = m.fileViewCache[:n]
	} else {
		m.fileViewCache = make([]fileView, n)
	}
	if cap(m.fileViewStarts) >= n+1 {
		m.fileViewStarts = m.fileViewStarts[:n+1]
	} else {
		m.fileViewStarts = make([]int, n+1)
	}
	m.fileViewStarts[0] = 0
	for i, f := range files {
		key := m.sectionCacheKey(f)
		fv := &m.fileViewCache[i]
		fv.file = f
		rendered, hasCached := m.sectionCache[key]
		fv.rendered = rendered
		fv.hasCached = hasCached
		fv.offsets = m.sectionLineOffsets[key]
		if lc, ok := m.sectionLineCache[key]; ok {
			fv.lineCount = lc
		} else if hasCached {
			lc = strings.Count(rendered, "\n")
			if len(rendered) == 0 || rendered[len(rendered)-1] != '\n' {
				lc++
			}
			fv.lineCount = max(lc, 1)
		} else {
			fv.lineCount = m.sectionLineCountEstimate(f)
		}
		m.fileViewStarts[i+1] = m.fileViewStarts[i] + fv.lineCount
	}
	m.fileViewDirty = false
}

// renderDiffPane renders only the diff lines visible in the current viewport,
// skipping file sections that lie entirely outside the scroll window.
// The result is cached by (scrollTop, sectionCache state) — cursor-blink renders
// that don't change scroll position or sections return in O(1) with zero allocs.
func (m *model) renderDiffPane() string {
	// Fast path: same scroll position and no section cache changes since last render.
	if m.diffPaneCache != "" && m.scrollTop == m.diffPaneScrollTop {
		return m.diffPaneCache
	}

	if m.isLoading || m.loadErr != "" {
		msg := "Loading..."
		if m.loadErr != "" {
			msg = "Error: " + m.loadErr
		}
		// Manual padding: Width(n).Render(text) undercounts EA Ambiguous chars in
		// loadErr, producing DiffPaneWidth+K-wide rows under Japanese locale.
		paneW := m.layout.DiffPaneWidth
		truncatedMsg := textutil.TruncateColumns(msg, paneW)
		padded := truncatedMsg + strings.Repeat(" ", max(paneW-textutil.VisibleWidth(truncatedMsg), 0))
		loadLine := overlay.EmptyStyle.Render(padded)
		blank := overlay.EmptyStyle.Render(strings.Repeat(" ", paneW))
		var lsb strings.Builder
		lsb.WriteString(loadLine)
		for i := 1; i < m.bodyHeight(); i++ {
			lsb.WriteByte('\n')
			lsb.WriteString(blank)
		}
		return lsb.String()
	}
	// Rebuild the per-file view cache if stale. This resolves sectionCacheKey,
	// rendered section, line offsets, and line count once per dirty cycle instead
	// of doing O(N) hash-map lookups on every renderDiffPane call.
	if m.fileViewDirty || len(m.fileViewCache) == 0 {
		m.rebuildFileViewCache()
	}
	views := m.fileViewCache
	if len(views) == 0 {
		return overlay.EmptyStyle.Width(m.layout.DiffPaneWidth).Height(m.bodyHeight()).
			Render("No changes to review.")
	}

	bodyH := m.bodyHeight()
	scrollTop := m.scrollTop
	scrollEnd := scrollTop + bodyH

	sb := &m.diffPaneBuf
	sb.Reset()
	sb.Grow(bodyH * (m.layout.DiffPaneWidth + 1))
	lineOffset := 0   // absolute line index of the first line of the current file section
	linesWritten := 0 // lines written to sb so far

	// Sticky file header: use binary search on precomputed fileViewStarts to find
	// in O(log N) the file whose header has scrolled above the viewport, instead of
	// scanning all files from the beginning on every scroll event.
	{
		starts := m.fileViewStarts
		// Find the last file whose start < scrollTop: that file's header might be sticky.
		// starts[i+1] = starts[i] + views[i].lineCount; find i where starts[i] < scrollTop <= starts[i+1].
		lo, hi := 0, len(views)
		for lo < hi {
			mid := lo + (hi-lo)/2
			if starts[mid+1] <= scrollTop {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		// lo = first file whose section contains or follows scrollTop.
		// Check if it actually has its header scrolled above the viewport.
		if lo < len(views) && starts[lo] < scrollTop && starts[lo+1] > scrollTop {
			fv := &views[lo]
			if fv.hasCached {
				var headerLine string
				if len(fv.offsets) > 1 {
					headerLine = fv.rendered[:int(fv.offsets[1])]
				} else if nl := strings.IndexByte(fv.rendered, '\n'); nl >= 0 {
					headerLine = fv.rendered[:nl+1]
				} else {
					headerLine = fv.rendered
				}
				if len(headerLine) > 0 {
					sb.WriteString(headerLine)
					if headerLine[len(headerLine)-1] != '\n' {
						sb.WriteByte('\n')
					}
					linesWritten = 1
					scrollEnd-- // one row occupied by sticky header
				}
			} else {
				rs := m.styles()
				w := m.layout.DiffContentWidth
				if w <= 0 {
					w = 40
				}
				pathText := "  " + textutil.TruncateColumns(fv.file.Path, w-2)
				rs.RawSbGroup.WriteWidthTo(sb, pathText, w)
				sb.WriteByte('\n')
				linesWritten = 1
				scrollEnd--
			}
		}
	}

	for i := range views {
		fv := &views[i]
		rendered := fv.rendered
		hasCached := fv.hasCached
		lineCount := fv.lineCount
		fileStart := lineOffset
		fileEnd := lineOffset + lineCount
		lineOffset = fileEnd

		if fileEnd <= scrollTop {
			// Entirely above viewport — skip rendering.
			continue
		}
		if fileStart >= scrollEnd {
			// Entirely below viewport — stop.
			break
		}

		// relStart / relEnd: line indices within this section that fall in [scrollTop, scrollEnd).
		relStart := max(0, scrollTop-fileStart)
		relEnd := min(scrollEnd-fileStart, lineCount)

		if !hasCached {
			// Placeholder: section not yet pre-rendered (prewarm in-flight). Write
			// only the visible viewport lines directly into sb — avoids allocating the
			// full placeholder string and byte-scanning it for the viewport slice.
			rs := m.styles()
			w := m.layout.DiffContentWidth
			if w <= 0 {
				w = 40
			}
			cur := relStart
			if cur == 0 && relEnd > 0 {
				pathText := "  " + textutil.TruncateColumns(fv.file.Path, w-2)
				rs.RawSbGroup.WriteWidthTo(sb, pathText, w)
				sb.WriteByte('\n')
				linesWritten++
				cur = 1
			}
			for ; cur < relEnd && linesWritten < bodyH; cur++ {
				rs.RawSbEmpty.WriteOpen(sb)
				textutil.WriteSpaces(sb, w)
				rs.RawSbEmpty.WriteClose(sb)
				sb.WriteByte('\n')
				linesWritten++
			}
			if linesWritten >= bodyH {
				break
			}
			continue
		}

		if offsets := fv.offsets; len(offsets) > 0 {
			// Zero-copy fast path: compute the byte range for [relStart, relEnd) and
			// write the entire visible chunk as ONE WriteString instead of iterating
			// line by line. This replaces up to bodyH individual WriteString calls with
			// a single memmove of the contiguous byte range, giving a large speedup
			// when most visible lines come from a single fully-cached section.
			end := min(relEnd, len(offsets))
			wantLines := min(end-relStart, bodyH-linesWritten)
			if wantLines > 0 {
				endLine := relStart + wantLines
				byteStart := int(offsets[relStart])
				var byteEnd int
				if endLine < len(offsets) {
					byteEnd = int(offsets[endLine])
				} else {
					byteEnd = len(rendered)
				}
				chunk := rendered[byteStart:byteEnd]
				sb.WriteString(chunk)
				if len(chunk) > 0 && chunk[len(chunk)-1] != '\n' {
					sb.WriteByte('\n')
				}
				linesWritten += wantLines
			}
		} else {
			// Slow path: byte scan (used when sectionLineOffsets hasn't been populated yet).
			localLine := 0
			pos := 0
			for pos <= len(rendered) {
				nl := strings.IndexByte(rendered[pos:], '\n')
				var line string
				if nl < 0 {
					line = rendered[pos:]
					pos = len(rendered) + 1
				} else {
					line = rendered[pos : pos+nl+1]
					pos += nl + 1
				}
				if line == "" {
					continue
				}
				if localLine >= relStart && localLine < relEnd {
					sb.WriteString(line)
					if len(line) == 0 || line[len(line)-1] != '\n' {
						sb.WriteByte('\n')
					}
					linesWritten++
				}
				localLine++
				if localLine >= relEnd {
					break
				}
			}
		}
		if linesWritten >= bodyH {
			break
		}
	}

	// Pad to bodyH lines (cachedPadLine is computed lazily, invalidated by clearRenderCache).
	if m.cachedPadLine == "" {
		rs := m.styles()
		padBuf := textutil.AcquireBuilder()
		rs.RawSbEmpty.WriteOpen(padBuf)
		textutil.WriteSpaces(padBuf, m.layout.DiffPaneWidth)
		rs.RawSbEmpty.WriteClose(padBuf)
		padBuf.WriteByte('\n')
		m.cachedPadLine = padBuf.String()
		textutil.ReleaseBuilder(padBuf)
	}
	for linesWritten < bodyH {
		sb.WriteString(m.cachedPadLine)
		linesWritten++
	}

	// Zero-alloc result: share the buffer's backing array instead of copying via String().
	// sb.Bytes() returns the underlying slice without allocating; unsafe.String wraps it
	// into a string header with zero copy. The backing array lives in m.diffPaneBuf and
	// remains valid as long as diffPaneCache holds the reference — it is only invalidated
	// by clearRenderCache which sets diffPaneCache="" before the next sb.Reset().
	raw := sb.Bytes()
	n := len(raw)
	if n > 0 && raw[n-1] == '\n' {
		n--
	}
	result := unsafe.String(unsafe.SliceData(raw), n)
	m.diffPaneCache = result
	m.diffPaneScrollTop = scrollTop
	return result
}
