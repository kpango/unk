package model

import (
	"bytes"

	"github.com/kpango/unk/internal/tui/patch"
	"github.com/kpango/unk/internal/tui/render"
	"github.com/kpango/unk/internal/types"
)

// newSplitConfig builds a render.SplitConfig from the model's current state.
func (m *model) newSplitConfig() render.SplitConfig {
	return render.SplitConfig{
		RS:               m.styles(),
		Palette:          m.palette(),
		DiffContentWidth: m.layout.DiffContentWidth,
		ShowLineNumbers:  m.showLineNumbers,
		ThemeID:          m.themeID,
		CodeHOffset:      m.codeHorizontalOffset,
		ShowUnkHeaders:   m.showUnkHeaders,
		ShowAgentNotes:   m.showAgentNotes,
	}
}

// renderPatchSplitInto renders a side-by-side split diff and writes the result
// into sb. For ≥ render.MinParallelUnks unks the work is dispatched to goroutines
// (renderSplitParallel); for 1–3 unks all state lives on the stack with zero
// closure/heap allocations (sequential path below).
//
// Fast path (0–1 @@ header): avoids patch.SplitUnks entirely — the patch.Segment
// is built inline on the stack, eliminating the last remaining allocation in
// the uncached render path. The inline scan detects the first @@ to extract
// startOld/startNew and breaks early on the second @@ so it is never slower
// than O(first-unk-lines) for the common single-unk case.
func (m *model) renderPatchSplitInto(sb *bytes.Buffer, f types.DiffFile, lines []string) {
	// Inline scan: find the first @@ header and check whether there are more.
	firstIdx := -1
	nUnks := 0
	var firstHeaderLine string
	for i, l := range lines {
		if len(l) == 0 || l[0] != '@' {
			continue
		}
		nUnks++
		if nUnks == 1 {
			firstIdx = i
			firstHeaderLine = l
		} else {
			break // ≥2 unks detected; stop scanning
		}
	}

	if nUnks <= 1 {
		// Zero-alloc path: single (or no) unk fits entirely in one patch.Segment on the stack.
		var seg patch.Segment
		seg.Lines = lines
		if firstIdx >= 0 {
			o, n := patch.ParseUnkHeader(firstHeaderLine)
			if o > 0 {
				seg.StartOld = o - 1
			}
			if n > 0 {
				seg.StartNew = n - 1
			}
			seg.FirstLineIdx = firstIdx
		}
		var r render.SplitRenderer
		r.Init(m.newSplitConfig(), f, lines)
		r.RenderUnkCore(sb, seg)
		return
	}

	segs := patch.SplitUnks(lines)
	if len(segs) >= render.MinParallelUnks {
		m.renderSplitParallel(sb, f, lines, segs)
		return
	}
	// Sequential path: SplitRenderer lives on the stack — no heap allocations.
	var r render.SplitRenderer
	r.Init(m.newSplitConfig(), f, lines)
	for _, seg := range segs {
		r.RenderUnkCore(sb, seg)
	}
}

// renderSplitParallel handles ≥ render.MinParallelUnks unks by heap-allocating
// SplitRenderer (once) and rendering each unk in a separate goroutine.
// NoIC is set so goroutines do not read/write the shared icCtx/icDel/icAdd fields
// (which are goroutine-local and would race across concurrent RenderUnkCore calls).
func (m *model) renderSplitParallel(sb *bytes.Buffer, f types.DiffFile, lines []string, segs []patch.Segment) {
	r := &render.SplitRenderer{}
	r.Init(m.newSplitConfig(), f, lines)
	r.SetNoIC(true) // disable inline IC; parallel goroutines share r and must not race
	render.RenderUnksParallelInto(sb, segs, r.RenderUnkInto)
}
