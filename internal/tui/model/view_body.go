package model

import (
	"unsafe"

	"github.com/kpango/unk/internal/tui/layout"
)

// renderBody renders sidebar + divider + diff pane + scrollbar, or just diff pane +
// scrollbar when the sidebar is hidden. The combined result (including scrollbar) is
// stored in m.bodyCache so View() returns in O(1) with zero allocations on cache hits.
func (m *model) renderBody() string {
	bodyH := m.bodyHeight()
	// Capture before any rendering that might mutate these fields.
	diffPaneCached := m.diffPaneCache != "" && m.scrollTop == m.diffPaneScrollTop
	// When the sidebar is hidden renderSidebar() is never called so sidebarRowsDirty
	// is never reset. Treat the no-sidebar case as always stable so bodyCache is used.
	sidebarStable := !m.layout.RenderSidebar || !m.sidebarRowsDirty

	// Fast path: body cache includes diffPane + scrollbar (+ sidebar when visible).
	// diffPaneCached ensures the scroll position and section content haven't changed.
	if diffPaneCached && sidebarStable && m.focusArea == FocusFiles && m.bodyCache != "" {
		return m.bodyCache
	}

	rs := m.styles()
	diffPane := m.renderDiffPane()
	scrollbar := m.renderScrollbar(bodyH)

	sb := &m.bodyBuf
	sb.Reset()
	if !m.layout.RenderSidebar {
		layout.JoinColumnsInto(sb, "", diffPane, scrollbar, bodyH)
	} else {
		if m.divCharCache == "" {
			m.divCharCache = rs.DivStr
		}
		sidebar := m.renderSidebar()
		layout.JoinColumnsAllInto(sb, sidebar, m.divCharCache, diffPane, scrollbar, bodyH)
	}
	// Zero-alloc: share bodyBuf's backing array instead of copying via String().
	// The backing array lives in m.bodyBuf and is valid until the next sb.Reset()
	// (which only runs on cache miss). On cache hit the fast path returns bodyCache
	// without calling Reset(), so the backing array remains intact.
	raw := sb.Bytes()
	result := unsafe.String(unsafe.SliceData(raw), len(raw))

	if m.focusArea == FocusFiles {
		m.bodyCache = result
	} else {
		m.bodyCache = ""
	}
	return result
}
