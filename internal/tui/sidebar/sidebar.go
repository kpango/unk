package sidebar

import (
	"bytes"
	"strings"

	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/tui/styles"
	"github.com/kpango/unk/internal/tui/textutil"
	"github.com/kpango/unk/internal/types"
)

// Entry is one row in the sidebar (either a directory group header or a file row).
// Row bytes live in Model.sidebarFlatBuf; RowStart/RowSplit/RowEnd are int32 offsets into it:
//
//	sidebarFlatBuf[RowStart:RowSplit]  = unselected row ANSI bytes
//	sidebarFlatBuf[RowSplit:RowEnd]    = selected row ANSI bytes (empty for group rows)
type Entry struct {
	IsGroup  bool
	Label    string // group: directory label; file: display name
	FileIdx  int    // -1 for groups; index into files slice for file rows
	RowStart int32  // start offset in sidebarFlatBuf
	RowSplit int32  // boundary: unsel ends / sel begins
	RowEnd   int32  // end offset in sidebarFlatBuf
}

// FileStateIcon returns the icon character and theme color for a file's change type.
func FileStateIcon(f types.DiffFile, p styles.Palette) (icon string, color string) {
	if f.IsUntracked {
		return "?", p.FileUntracked
	}
	switch f.Metadata.Type {
	case types.FileChangeNew:
		return "A", p.FileNew
	case types.FileChangeDeleted:
		return "D", p.FileDeleted
	case types.FileChangeRenamePure, types.FileChangeRenameChanged:
		return "R", p.FileRenamed
	default:
		return "M", p.FileModified
	}
}

// ScrollTop computes the scroll offset used by renderSidebarInner and
// the mouse click handler to keep the selected entry visible. It must be called
// with the same entries and bodyH as the last render to produce the same offset.
func ScrollTop(entries []Entry, selectedFileIndex, bodyH int) int {
	visibleRows := max(bodyH-1, 1)
	selectedRow := -1
	for i, e := range entries {
		if !e.IsGroup && e.FileIdx == selectedFileIndex {
			selectedRow = i
			break
		}
	}
	if selectedRow < 0 || len(entries) <= visibleRows {
		return 0
	}
	top := max(selectedRow-visibleRows/2, 0)
	if top+visibleRows > len(entries) {
		top = len(entries) - visibleRows
	}
	return top
}

// BuildEntries groups files by directory for sidebar rendering.
// Files in root (no directory) have no group header.
// Files with the same directory are grouped under one header.
// The display name for each file is just its basename (or "prev -> next" for renames).
func BuildEntries(into []Entry, files []types.DiffFile) []Entry {
	// Reuse backing array if caller provides one with sufficient capacity;
	// otherwise allocate 2× len(files): worst case is one group header per file.
	entries := into[:0]
	if cap(entries) < len(files)*2 {
		entries = make([]Entry, 0, len(files)*2)
	}
	activeGroup := ""

	for i, f := range files {
		path := f.Path
		// Extract directory portion.
		slash := strings.LastIndexByte(path, '/')
		dir := ""
		name := path
		if slash >= 0 {
			dir = path[:slash]
			name = path[slash+1:]
		}

		// Emit group header on directory change (skip root-level files).
		if dir != activeGroup {
			activeGroup = dir
			if dir != "" {
				// path[:slash+1] is dir+"/" as a zero-copy re-slice of path.
				entries = append(entries, Entry{IsGroup: true, Label: path[:slash+1], FileIdx: -1})
			}
		}

		// Build display name: basename, or "prev -> next" for renames with different base names.
		displayName := name
		if f.PreviousPath != nil && *f.PreviousPath != "" && *f.PreviousPath != f.Path {
			prevSlash := strings.LastIndexByte(*f.PreviousPath, '/')
			prevName := *f.PreviousPath
			if prevSlash >= 0 {
				prevName = (*f.PreviousPath)[prevSlash+1:]
			}
			if prevName != name {
				displayName = prevName + " -> " + name
			}
		}

		entries = append(entries, Entry{IsGroup: false, Label: displayName, FileIdx: i})
	}
	return entries
}

// WriteGroupRow writes a single group-header row into sb without allocating.
func WriteGroupRow(sb *bytes.Buffer, rs *styles.RendererStyles, label string, sw int) {
	labelW := textutil.VisibleWidth(label)
	rs.RawSbGroup.WriteOpen(sb)
	sb.WriteByte(' ')
	if labelW > sw-2 {
		truncated := textutil.TruncateColumns(label, sw-3)
		truncW := textutil.VisibleWidth(truncated)
		sb.WriteString(truncated)
		sb.WriteString("…")
		textutil.WriteSpaces(sb, sw-1-truncW-layout.EllipsisW)
	} else {
		sb.WriteString(label)
		textutil.WriteSpaces(sb, sw-1-labelW)
	}
	rs.RawSbGroup.WriteClose(sb)
}

// WriteFileRow writes a single file row (selected or not) into sb without allocating.
func WriteFileRow(sb *bytes.Buffer, rs *styles.RendererStyles, p styles.Palette, f types.DiffFile, label string, isSelected bool, sw int) {
	si := 0
	rawStrip := rs.RawSbStripOff
	rawBase := rs.RawSbRowBase
	rawMuted := rs.RawSbMuted
	rawAdd := rs.RawSbAdd
	rawDel := rs.RawSbDel
	if isSelected {
		si = 1
		rawStrip = rs.RawSbStrip
		rawBase = rs.RawSbRowSel
		rawMuted = rs.RawSbMutedSel
		rawAdd = rs.RawSbAddSel
		rawDel = rs.RawSbDelSel
	}

	icon, iconColor := FileStateIcon(f, p)
	rawIcon := styles.SidebarIconRaw(iconColor, si, p, rs)

	hasAdd := f.Stats.Additions > 0
	hasDel := f.Stats.Deletions > 0
	statsW := 0
	if hasAdd {
		statsW = 1 + textutil.CountDigits(f.Stats.Additions)
		if f.StatsTruncated {
			statsW++
		}
	}
	if hasDel {
		if hasAdd {
			statsW++
		}
		statsW += 1 + textutil.CountDigits(f.Stats.Deletions)
	}

	const rowPrefixW = 4
	nameW := max(sw-rowPrefixW-statsW, 1)
	name := label
	if textutil.VisibleWidth(name) > nameW {
		name = "…" + textutil.TruncateNameSuffix(name, nameW-layout.EllipsisW)
	}

	rawStrip.WriteTo(sb, " ")
	rawBase.WriteTo(sb, " ")
	rawIcon.WriteTo(sb, icon)
	rawBase.WriteTo(sb, " ")
	rawMuted.WriteWidthTo(sb, name, nameW)
	if hasAdd {
		rawAdd.WriteOpen(sb)
		sb.WriteByte('+')
		textutil.WriteDecimalInt(sb, f.Stats.Additions)
		if f.StatsTruncated {
			sb.WriteByte('+')
		}
		rawAdd.WriteClose(sb)
		if hasDel {
			rawMuted.WriteTo(sb, " ")
		}
	}
	if hasDel {
		rawDel.WriteOpen(sb)
		sb.WriteByte('-')
		textutil.WriteDecimalInt(sb, f.Stats.Deletions)
		rawDel.WriteClose(sb)
	}
}
