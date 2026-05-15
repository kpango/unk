package model

import "github.com/kpango/unk/internal/tui/overlay"

// renderFull is the complete View() assembly: menu bar + body + status bar +
// optional overlays. Called synchronously in Update() on every event when
// sectionCache is populated. At ~10–50 µs it comfortably fits within the
// BubbleTea frame budget on all supported platforms.
func (m *model) renderFull() string {
	if m.termWidth == 0 {
		return ""
	}
	sb := &m.frameBuf
	sb.Reset()
	if !m.pagerMode {
		sb.WriteString(m.renderMenuBar())
		sb.WriteByte('\n')
	}
	sb.WriteString(m.renderBody())
	if !m.pagerMode {
		sb.WriteByte('\n')
		sb.WriteString(m.renderStatusBar())
	}
	if m.showHelp && !m.pagerMode {
		return overlay.Help(sb.String(), m.termWidth, m.termHeight, m.keys, m.helpPage)
	}
	if m.showKeymapList && !m.pagerMode {
		return overlay.KeymapList(sb.String(), m.termWidth, m.termHeight, m.keymapStyle, m.keymapListIdx)
	}
	if m.menuOpen && !m.pagerMode {
		return overlayMenu(sb.String(), m.termWidth, m.termHeight, m)
	}
	return sb.String()
}
