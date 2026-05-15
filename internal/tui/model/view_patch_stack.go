package model

import (
	"bytes"

	"github.com/kpango/unk/internal/tui/render"
	"github.com/kpango/unk/internal/types"
)

// newStackConfig builds a render.StackConfig from the model's current state.
func (m *model) newStackConfig() render.StackConfig {
	return render.StackConfig{
		RS:               m.styles(),
		DiffContentWidth: m.layout.DiffContentWidth,
		ShowLineNumbers:  m.showLineNumbers,
		WrapLines:        m.wrapLines,
		CodeHOffset:      m.codeHorizontalOffset,
		ShowUnkHeaders:   m.showUnkHeaders,
		ShowAgentNotes:   m.showAgentNotes,
	}
}

// renderPatchStackInto renders a stacked diff and writes the result into sb.
// Uses a pooled StackRenderer so slice capacities and the sep string survive across renders.
func (m *model) renderPatchStackInto(sb *bytes.Buffer, f types.DiffFile, lines []string) {
	r := render.AcquireStackRenderer()
	r.Init(m.newStackConfig(), f, lines)
	r.RenderInto(sb)
	render.ReleaseStackRenderer(r)
}
