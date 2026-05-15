package model

import (
	"fmt"

	"github.com/kpango/unk/internal/tui/textutil"
)

// renderStatusBar renders the bottom chrome row: filter input, filter summary, or notice text.
// The rendered result is cached except when in FocusFilter mode (cursor changes every frame).
func (m *model) renderStatusBar() string {
	if m.focusArea == FocusFilter || m.focusArea == FocusSearch || m.focusArea == FocusCommand {
		// Cursor blink changes every frame; mark dirty so the next FocusFiles frame re-renders.
		m.statusBarDirty = true
		return m.renderStatusBarInner()
	}
	if !m.statusBarDirty && m.statusBarCache != "" {
		return m.statusBarCache
	}
	result := m.renderStatusBarInner()
	m.statusBarCache = result
	m.statusBarDirty = false
	return result
}

func (m *model) renderStatusBarInner() string {
	rs := m.styles()
	// StatusBar has Padding(0,1) — 1 col each side. Use manual padding (visibleWidth-based)
	// so EA Ambiguous chars (filter text, notice text) don't overflow termWidth.
	innerW := max(m.termWidth-2, 0)

	sb := &m.statusBarBuf
	renderBar := func(text string) string {
		sb.Reset()
		rs.RawStatusBar.WriteWidthTo(sb, textutil.TruncateColumns(text, innerW), innerW)
		return sb.String()
	}

	if m.focusArea == FocusFilter {
		return renderBar("filter: " + m.filter.View())
	}
	if m.focusArea == FocusSearch {
		return renderBar("search: " + m.search.View())
	}
	if m.focusArea == FocusCommand {
		return renderBar(":" + m.cmdInput.View())
	}
	filterVal := m.filter.Value()
	if filterVal != "" {
		return renderBar("filter=" + filterVal)
	}
	if m.grepQuery != "" {
		info := "search=" + m.grepQuery
		if len(m.grepMatchLines) > 0 {
			info += fmt.Sprintf("  [%d/%d]", m.grepMatchIdx+1, len(m.grepMatchLines))
		} else {
			info += "  [no matches]"
		}
		return renderBar(info)
	}
	if m.updateNotice != "" {
		return renderBar(m.updateNotice)
	}
	// Idle: show file position, unk position, and scroll percentage.
	files := m.visibleFiles()
	nFiles := len(files)
	ctx := ""
	if nFiles > 0 {
		filePos := fmt.Sprintf("file %d/%d", m.selectedFileIndex+1, nFiles)
		unkPos := ""
		if m.selectedFileIndex < len(files) {
			nUnks := len(files[m.selectedFileIndex].Metadata.Unks)
			if nUnks > 0 {
				unkPos = fmt.Sprintf("  unk %d/%d", m.selectedUnkIndex+1, nUnks)
			}
		}
		totalLines := m.totalDiffLines()
		bodyH := m.bodyHeight()
		pct := 0
		if totalLines > bodyH && bodyH > 0 {
			pct = m.scrollTop * 100 / (totalLines - bodyH)
		} else if totalLines <= bodyH {
			pct = 100
		}
		ctx = fmt.Sprintf("%s%s  %d%%", filePos, unkPos, pct)
	}
	return renderBar(ctx)
}
