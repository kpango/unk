package model

import (
	"github.com/kpango/unk/internal/tui/sidebar"
)

// renderSidebar renders the file list with directory groups, basename display,
// 1px accent selection strip, and viewport clamping to bodyHeight.
func (m *model) renderSidebar() string {
	// The sidebar file rows are expensive to re-render (many lipgloss calls).
	// They only change when selection, theme, layout, filter text, file list,
	// or live comment counts change — all of which set sidebarRowsDirty=true.
	if !m.sidebarRowsDirty && m.sidebarRowsCache != "" {
		return m.sidebarRowsCache
	}
	result := m.renderSidebarInner()
	m.sidebarRowsCache = result
	m.sidebarRowsDirty = false
	return result
}

func (m *model) renderSidebarInner() string {
	rs := m.styles()
	p := m.palette()
	files := m.visibleFiles()
	sw := m.layout.SidebarWidth

	// Rebuild entries and pre-build per-entry ANSI row bytes when the file
	// list or sidebar width changes. All row bytes are packed into sidebarFlatBuf
	// (one slab alloc); sidebar.Entry holds int32 offsets — no per-entry string alloc.
	if m.sidebarEntriesDirty || m.sidebarEntries == nil || m.sidebarEntriesWidth != sw {
		// Reuse sidebarEntries backing array only on the main goroutine where it
		// is exclusively owned. Background render clones share the same backing
		// array via the copied slice header, so pass nil to force a fresh alloc.
		var reuseSlice []sidebar.Entry
		if m.isMainGoroutine {
			reuseSlice = m.sidebarEntries
		}
		entries := sidebar.BuildEntries(reuseSlice, files)
		// Estimate flat buffer size: ~(sw*4+32) bytes per row, 2 rows per file entry.
		estSize := len(entries) * (sw*4 + 32) * 2
		flat := m.sidebarFlatBuf[:0]
		if cap(flat) < estSize {
			flat = make([]byte, 0, estSize)
		}
		rowBuf := &m.sidebarEntryBuf
		for i := range entries {
			e := &entries[i]
			rowBuf.Reset()
			start := int32(len(flat))
			if e.IsGroup {
				sidebar.WriteGroupRow(rowBuf, rs, e.Label, sw)
				flat = append(flat, rowBuf.Bytes()...)
				split := int32(len(flat))
				e.RowStart = start
				e.RowSplit = split
				e.RowEnd = split
			} else {
				f := files[e.FileIdx]
				sidebar.WriteFileRow(rowBuf, rs, p, f, e.Label, false, sw)
				flat = append(flat, rowBuf.Bytes()...)
				split := int32(len(flat))
				rowBuf.Reset()
				sidebar.WriteFileRow(rowBuf, rs, p, f, e.Label, true, sw)
				flat = append(flat, rowBuf.Bytes()...)
				e.RowStart = start
				e.RowSplit = split
				e.RowEnd = int32(len(flat))
			}
		}
		m.sidebarFlatBuf = flat
		m.sidebarEntries = entries
		m.sidebarEntriesDirty = false
		m.sidebarEntriesWidth = sw
	}

	entries := m.sidebarEntries
	bodyH := m.bodyHeight()
	visibleRows := max(bodyH, 1)
	scrollTop := sidebar.ScrollTop(entries, m.selectedFileIndex, bodyH)

	visible := entries
	if scrollTop < len(entries) {
		visible = entries[scrollTop:]
	}
	if len(visible) > visibleRows {
		visible = visible[:visibleRows]
	}

	// Assemble from pre-built row strings. Use the model-local buffer to avoid
	// sync.Pool round-trips (pool contention under high parallelism is expensive).
	sb := &m.sidebarBuf
	sb.Reset()
	sb.Grow(len(visible) * (sw*3 + 4))

	flat := m.sidebarFlatBuf
	rowsWritten := 0
	sel := m.selectedFileIndex
	for i := range visible {
		if rowsWritten > 0 {
			sb.WriteByte('\n')
		}
		rowsWritten++
		e := &visible[i]
		if e.IsGroup || e.FileIdx != sel {
			sb.Write(flat[e.RowStart:e.RowSplit])
		} else {
			sb.Write(flat[e.RowSplit:e.RowEnd])
		}
	}

	for rowsWritten < visibleRows {
		if rowsWritten > 0 {
			sb.WriteByte('\n')
		}
		rs.RawSbEmpty.WriteWidthTo(sb, "", sw)
		rowsWritten++
	}

	return sb.String()
}
