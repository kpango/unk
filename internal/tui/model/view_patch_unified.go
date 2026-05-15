package model

import (
	"bytes"

	"github.com/kpango/unk/internal/tui/patch"
	"github.com/kpango/unk/internal/tui/render"
	"github.com/kpango/unk/internal/types"
)

// newUnifiedConfig builds a render.UnifiedConfig from the model's current state.
func (m *model) newUnifiedConfig() render.UnifiedConfig {
	return render.UnifiedConfig{
		RS:               m.styles(),
		Palette:          m.palette(),
		DiffContentWidth: m.layout.DiffContentWidth,
		ShowLineNumbers:  m.showLineNumbers,
		WrapLines:        m.wrapLines,
		ThemeID:          m.themeID,
		CodeHOffset:      m.codeHorizontalOffset,
		ShowUnkHeaders:   m.showUnkHeaders,
		ShowAgentNotes:   m.showAgentNotes,
	}
}

// renderPatchUnifiedInto renders the unified diff for one file and writes the result
// into sb. For files with many unks (≥ render.MinParallelUnks) each unk is rendered
// in a separate goroutine.
func (m *model) renderPatchUnifiedInto(sb *bytes.Buffer, f types.DiffFile, lines []string) {
	cacheKey := f.Metadata.CacheKey
	if cacheKey == "" {
		cacheKey = f.ID
	}

	segs := patch.SplitUnks(lines)
	if len(segs) >= render.MinParallelUnks {
		r := &render.UnifiedRenderer{}
		r.Init(m.newUnifiedConfig(), f, lines, m.intraCache[cacheKey])
		render.RenderUnksParallelInto(sb, segs, r.RenderUnkInto)
		return
	}
	if len(segs) == 0 {
		segs = []patch.Segment{{Lines: lines, FirstLineIdx: 0}}
	}
	var r render.UnifiedRenderer
	r.Init(m.newUnifiedConfig(), f, lines, m.intraCache[cacheKey])
	if len(segs) == 1 {
		r.RenderUnkCore(sb, segs[0])
		return
	}
	for _, seg := range segs {
		r.RenderUnkInto(seg, sb)
	}
}
