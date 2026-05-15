package model

import (
	"cmp"

	"github.com/kpango/unk/internal/tui/textutil"
)

// renderMenuBar renders the single-row top chrome.
func (m *model) renderMenuBar() string {
	if !m.menuBarDirty && m.menuBarCache != "" {
		return m.menuBarCache
	}
	result := m.renderMenuBarInner()
	m.menuBarCache = result
	m.menuBarDirty = false
	return result
}

func (m *model) renderMenuBarInner() string {
	rs := m.styles()

	title := cmp.Or(m.bootstrap.Changeset.Title, m.bootstrap.Changeset.SourceLabel)

	var totalAdd, totalDel int
	for _, f := range m.visibleFiles() {
		totalAdd += f.Stats.Additions
		totalDel += f.Stats.Deletions
	}

	// rightW: "+N  -M " — no ANSI, compute directly without lipgloss.Width.
	rightW := 5 + textutil.CountDigits(totalAdd) + textutil.CountDigits(totalDel)
	// titleStyle has Padding(0,1) = 2 extra cols; clamp title so left+right ≤ termWidth.
	maxTitleW := max(m.termWidth-rs.MenuLabelsW-rightW-2, 0)
	title = textutil.TruncateColumns(title, maxTitleW)
	titleVisW := textutil.VisibleWidth(title)

	gap := max(m.termWidth-rs.MenuLabelsW-titleVisW-2-rightW, 0)

	sb := &m.menuBarBuf
	sb.Reset()
	sb.Grow(m.termWidth * 4)

	// Static labels (pre-built per theme).
	sb.WriteString(rs.MenuLabels)
	// Title with Padding(0,1) baked into styles.RawStyle open/close.
	rs.RawMenuTitle.WriteTo(sb, title)
	// Gap fill with MenuBar background.
	rs.RawMenuBar.WriteOpen(sb)
	textutil.WriteSpaces(sb, gap)
	rs.RawMenuBar.WriteClose(sb)
	// Right: "+N  -M "
	rs.RawMenuAdd.WriteOpen(sb)
	sb.WriteByte('+')
	textutil.WriteDecimalInt(sb, totalAdd)
	rs.RawMenuAdd.WriteClose(sb)
	rs.RawMenuBar.WriteTo(sb, "  ")
	rs.RawMenuDel.WriteOpen(sb)
	sb.WriteByte('-')
	textutil.WriteDecimalInt(sb, totalDel)
	rs.RawMenuDel.WriteClose(sb)
	rs.RawMenuBar.WriteTo(sb, " ")

	return sb.String()
}
