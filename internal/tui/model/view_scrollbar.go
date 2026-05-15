package model

import (
	"bytes"
	"strings"
	"unsafe"

	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/tui/scrollbar"
	"github.com/kpango/unk/internal/tui/textutil"
)

// totalDiffLines returns the total terminal-line height of all visible file sections.
func (m *model) totalDiffLines() int {
	files := m.visibleFiles()
	total := 0
	for _, f := range files {
		key := m.sectionCacheKey(f)
		if lc, ok := m.sectionLineCache[key]; ok {
			total += lc
		} else if cached, ok := m.sectionCache[key]; ok {
			lc = strings.Count(cached, "\n")
			if len(cached) == 0 || cached[len(cached)-1] != '\n' {
				lc++
			}
			lc = max(lc, 1)
			m.sectionLineCache[key] = lc
			total += lc
		} else {
			total += m.sectionLineCountEstimate(f)
		}
	}
	return total
}

// scrollbarGeom computes the current scrollbar geometry using the pure
// scrollbar.Compute function. Returns (zero, false) when the content fits
// entirely in the viewport (no scrollbar needed).
func (m *model) scrollbarGeom() (scrollbar.Geom, bool) {
	return scrollbar.Compute(m.scrollTop, m.totalDiffLines(), m.bodyHeight())
}

// renderScrollbar produces a single-column string of bodyH lines showing
// a proportional thumb (█) on a track (│). When content fits in the viewport
// the track renders as blank space. The thumb brightens while being dragged.
func (m *model) renderScrollbar(bodyH int) string {
	rs := m.styles()
	sb := &m.scrollbarBuf
	sb.Reset()

	g, ok := m.scrollbarGeom()
	if !ok {
		// No scrollbar needed: render blank space with scrollbar-bg color.
		// Build one blank row then repeat — avoids a blankBuf Pool alloc.
		var blankBuf [64]byte
		tmp := bytes.NewBuffer(blankBuf[:0])
		rs.RawScrollBg.WriteOpen(tmp)
		textutil.WriteSpaces(tmp, layout.BoxCharW)
		rs.RawScrollBg.WriteClose(tmp)
		blank := tmp.Bytes()
		sb.Grow(bodyH * (tmp.Len() + 1))
		for i := range bodyH {
			if i > 0 {
				sb.WriteByte('\n')
			}
			sb.Write(blank)
		}
		return unsafe.String(unsafe.SliceData(sb.Bytes()), sb.Len())
	}

	trackStr := rs.ScrollTrackStr
	thumbStr := rs.ScrollThumbStr
	if m.scrollbarDragging {
		thumbStr = rs.ScrollThumbDragStr
	}
	sb.Grow(bodyH * (len(trackStr) + 1))
	for i := range bodyH {
		if i > 0 {
			sb.WriteByte('\n')
		}
		if i >= g.ThumbTop && i < g.ThumbTop+g.ThumbH {
			sb.WriteString(thumbStr)
		} else {
			sb.WriteString(trackStr)
		}
	}
	return unsafe.String(unsafe.SliceData(sb.Bytes()), sb.Len())
}
