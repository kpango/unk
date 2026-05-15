package model

import (
	"bytes"
	"slices"

	"github.com/charmbracelet/lipgloss"
	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/tui/textutil"
	"github.com/kpango/unk/internal/types"
)

// renderFileSection returns a pre-rendered section from sectionCache or, if the
// background prewarm hasn't completed yet, a lightweight placeholder. It never
// calls renderFileSectionInner synchronously, so View() is always O(1) and the
// UI goroutine is never blocked by syntax highlighting or lipgloss layout work.
// The placeholder is NOT stored in sectionCache so the next Update(prewarmMsg)
// cycle will show real content.
func (m *model) renderFileSection(f types.DiffFile, _ bool) string {
	if cached, ok := m.sectionCache[m.sectionCacheKey(f)]; ok {
		return cached
	}
	return m.renderFileSectionPlaceholder(f)
}

// renderFileSectionPlaceholder produces a correctly-sized skeleton for a file
// section that hasn't been pre-rendered yet. It shows the file path in the header
// row and blank lines for the diff body, using only string operations (no chroma,
// no complex lipgloss layout) so it completes in microseconds.
func (m *model) renderFileSectionPlaceholder(f types.DiffFile) string {
	n := m.sectionLineCountEstimate(f)
	rs := m.styles()
	w := m.layout.DiffContentWidth
	if w <= 0 {
		w = 40
	}

	pathText := "  " + textutil.TruncateColumns(f.Path, w-2)
	sb := &m.placeholderBuf
	sb.Reset()
	// Header row.
	rs.RawSbGroup.WriteWidthTo(sb, pathText, w)
	// Blank body lines: panel background + w spaces.
	for i := 1; i < n; i++ {
		sb.WriteByte('\n')
		rs.RawSbEmpty.WriteOpen(sb)
		textutil.WriteSpaces(sb, w)
		rs.RawSbEmpty.WriteClose(sb)
	}
	return sb.String()
}

// renderFileSectionInto writes the full section (header + diff body) into dest.
// Callers that can reuse a buffer (e.g. cmdPrewarmSections) should call this
// directly and avoid the String() copy paid by renderFileSectionInner.
func (m *model) renderFileSectionInto(dest *bytes.Buffer, f types.DiffFile, isSelected bool) {
	rs := m.styles()
	_ = isSelected

	// File header row.
	filePath := f.Path
	if f.PreviousPath != nil && *f.PreviousPath != "" && *f.PreviousPath != f.Path {
		filePath = *f.PreviousPath + " → " + f.Path
	}

	extra := ""
	if f.IsTooLarge {
		extra += "  [too large]"
	}
	if f.IsBinary {
		extra += "  [binary]"
	}

	// Compute visible widths without lipgloss — avoids termenv color lookups and
	// strings.Split calls inside lipgloss.Width / Style.Render.
	addStatW := 1 + textutil.CountDigits(f.Stats.Additions) // "+N"
	delStatW := 1 + textutil.CountDigits(f.Stats.Deletions) // "-N"
	extraW := textutil.VisibleWidth(extra)
	// Reserve: 2(prefix pad) + 2(suffix pad) + addStat + 1(sep) + delStat + extra.
	statsW := addStatW + 1 + delStatW + extraW
	maxPathW := max(m.layout.DiffContentWidth-statsW-4, 1)
	if textutil.VisibleWidth(filePath) > maxPathW {
		filePath = textutil.TruncateColumns(filePath, maxPathW-layout.EllipsisW) + "…"
	}
	// Use visibleWidth for filePath: lipgloss.Width miscounts East Asian Ambiguous
	// chars (e.g. "…") by 1 under Japanese locale, which would over-fill the header.
	used := 2 + textutil.VisibleWidth(filePath) + 2 + addStatW + 1 + delStatW + extraW
	fill := max(m.layout.DiffContentWidth-used, 0)

	// Header: "  filePath  +ADD -DEL[extra][fill]"
	rs.RawFileHeader.WriteOpen(dest)
	dest.WriteString("  ")
	dest.WriteString(filePath)
	dest.WriteString("  ")
	rs.RawFileHeader.WriteClose(dest)
	rs.RawFileAdd.WriteOpen(dest)
	dest.WriteByte('+')
	textutil.WriteDecimalInt(dest, f.Stats.Additions)
	rs.RawFileAdd.WriteClose(dest)
	rs.RawFileHeader.WriteTo(dest, " ")
	rs.RawFileDel.WriteOpen(dest)
	dest.WriteByte('-')
	textutil.WriteDecimalInt(dest, f.Stats.Deletions)
	rs.RawFileDel.WriteClose(dest)
	if extra != "" {
		rs.RawFileHeader.WriteTo(dest, extra)
	}
	if fill > 0 {
		rs.RawFileHeader.WriteOpen(dest)
		textutil.WriteSpaces(dest, fill)
		rs.RawFileHeader.WriteClose(dest)
	}
	dest.WriteByte('\n')

	// Skip body for binary/too-large files.
	if f.IsBinary || f.IsTooLarge {
		skipped := skippedStyle.Width(m.layout.DiffContentWidth).Render("  (content skipped)")
		dest.WriteString(skipped)
		return
	}

	m.renderPatchInto(dest, f)
}

// renderFileSectionInner renders a file section and returns it as a string.
// Use renderFileSectionInto when writing to an existing buffer to avoid the
// String() copy (e.g. in cmdPrewarmSections goroutines).
func (m *model) renderFileSectionInner(f types.DiffFile, isSelected bool) string {
	buf := textutil.AcquireBuilder()
	defer textutil.ReleaseBuilder(buf)
	m.renderFileSectionInto(buf, f, isSelected)
	return buf.String()
}

// renderPatchInto dispatches to the appropriate diff renderer and writes the
// result directly into sb. The patch is split into lines here once and shared.
//
// On the main goroutine (isMainGoroutine==true) it uses patchLinesCache to avoid
// re-splitting the same patch on every render. Background clones use the pooled
// path — safe because clones never share patchLinesCache with each other or the main.
func (m *model) renderPatchInto(sb *bytes.Buffer, f types.DiffFile) {
	if m.isMainGoroutine {
		key := f.Metadata.CacheKey
		if key == "" {
			key = f.ID
		}
		var lines []string
		var ok bool
		if m.patchLinesCache != nil {
			lines, ok = m.patchLinesCache[key]
		}
		if !ok {
			sp, l := textutil.AcquirePatchLines(f.Patch)
			lines = slices.Clone(l)
			textutil.ReleasePatchLines(sp)
			if m.patchLinesCache == nil {
				m.patchLinesCache = make(map[string][]string, 16)
			}
			m.patchLinesCache[key] = lines
		}
		switch m.layout.LayoutMode {
		case types.LayoutModeSplit:
			m.renderPatchSplitInto(sb, f, lines)
		case types.LayoutModeStack:
			m.renderPatchStackInto(sb, f, lines)
		default:
			m.renderPatchUnifiedInto(sb, f, lines)
		}
		return
	}
	// Background clone path: use pooled splitting to avoid shared mutable state.
	sp, lines := textutil.AcquirePatchLines(f.Patch)
	defer textutil.ReleasePatchLines(sp)
	switch m.layout.LayoutMode {
	case types.LayoutModeSplit:
		m.renderPatchSplitInto(sb, f, lines)
	case types.LayoutModeStack:
		m.renderPatchStackInto(sb, f, lines)
	default:
		m.renderPatchUnifiedInto(sb, f, lines)
	}
}

// skippedStyle is used in renderFileSectionInto for binary/too-large files.
var skippedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#666666")).
	Italic(true)
