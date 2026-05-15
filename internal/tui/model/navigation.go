package model

// navigation.go — file selection and unk/file navigation state mutations.

import (
	"strings"

	"github.com/kpango/unk/internal/tui/patch"
	"github.com/kpango/unk/internal/types"
)

// --- file selection ---

// rawVisibleFiles returns files filtered by the filename filter input only.
// It does not apply the grep/content-search filter.
func (m *model) rawVisibleFiles() []types.DiffFile {
	filterText := strings.ToLower(m.filter.Value())
	if filterText == "" {
		return m.bootstrap.Changeset.Files
	}
	var out []types.DiffFile
	for _, f := range m.bootstrap.Changeset.Files {
		if strings.Contains(strings.ToLower(f.Path), filterText) {
			out = append(out, f)
		}
	}
	return out
}

// visibleFiles returns files shown in the sidebar and diff pane.
// When a grep query is active and grepMatchFileSet is populated, only files
// with confirmed content matches (or not-yet-prewarmed files) are returned.
func (m *model) visibleFiles() []types.DiffFile {
	raw := m.rawVisibleFiles()
	if m.grepQuery == "" || m.grepMatchFileSet == nil {
		return raw
	}
	var out []types.DiffFile
	for i, f := range raw {
		if m.grepMatchFileSet[i] {
			out = append(out, f)
		}
	}
	return out
}

func (m *model) selectedFile() *types.DiffFile {
	files := m.visibleFiles()
	if m.selectedFileIndex < 0 || m.selectedFileIndex >= len(files) {
		return nil
	}
	f := files[m.selectedFileIndex]
	return &f
}

// --- unk navigation ---

func (m *model) navigateNextUnk() {
	files := m.visibleFiles()
	if len(files) == 0 {
		return
	}
	if m.selectedFileIndex >= len(files) {
		m.selectedFileIndex = len(files) - 1
		m.selectedUnkIndex = 0
	}
	f := files[m.selectedFileIndex]
	if m.selectedUnkIndex+1 < len(f.Metadata.Unks) {
		m.selectedUnkIndex++
	} else if m.selectedFileIndex+1 < len(files) {
		// Wrap to next file.
		m.selectedFileIndex++
		m.selectedUnkIndex = 0
	}
	m.scrollTop = m.computeUnkScrollOffset()
}

func (m *model) navigatePrevUnk() {
	if m.selectedUnkIndex > 0 {
		m.selectedUnkIndex--
	} else if m.selectedFileIndex > 0 {
		// Wrap to previous file.
		m.selectedFileIndex--
		files := m.visibleFiles()
		f := files[m.selectedFileIndex]
		m.selectedUnkIndex = max(0, len(f.Metadata.Unks)-1)
	}
	m.scrollTop = m.computeUnkScrollOffset()
}

func (m *model) navigateNextAnnotatedUnk() {
	files := m.visibleFiles()
	fi, hi := m.selectedFileIndex, m.selectedUnkIndex

	for {
		hi++
		if fi >= len(files) {
			break
		}
		if hi >= len(files[fi].Metadata.Unks) {
			fi++
			hi = 0
			if fi >= len(files) {
				break
			}
		}
		if patch.UnkHasAnnotation(files[fi], hi) {
			m.selectedFileIndex = fi
			m.selectedUnkIndex = hi
			m.scrollTop = m.computeUnkScrollOffset()
			return
		}
	}
}

func (m *model) navigatePrevAnnotatedUnk() {
	files := m.visibleFiles()
	fi := m.selectedFileIndex
	if fi >= len(files) {
		fi = len(files) - 1
	}
	hi := m.selectedUnkIndex

	for {
		hi--
		if hi < 0 {
			fi--
			if fi < 0 {
				break
			}
			n := len(files[fi].Metadata.Unks)
			if n == 0 {
				continue
			}
			hi = n - 1
		}
		if patch.UnkHasAnnotation(files[fi], hi) {
			m.selectedFileIndex = fi
			m.selectedUnkIndex = hi
			m.scrollTop = m.computeUnkScrollOffset()
			return
		}
	}
}

// navigateNextFile jumps to the first unk of the next file.
func (m *model) navigateNextFile() {
	files := m.visibleFiles()
	if m.selectedFileIndex+1 < len(files) {
		m.selectedFileIndex++
		m.selectedUnkIndex = 0
		m.scrollTop = m.computeUnkScrollOffset()
		m.markSidebarDirty()
	}
}

// navigatePrevFile jumps to the first unk of the previous file.
func (m *model) navigatePrevFile() {
	if m.selectedFileIndex > 0 {
		m.selectedFileIndex--
		m.selectedUnkIndex = 0
		m.scrollTop = m.computeUnkScrollOffset()
		m.markSidebarDirty()
	}
}

// --- scroll offset computation ---

// computeUnkScrollOffset returns the scrollTop value that places the selected unk near the top.
// It walks the rendered stream counting lines before the target file+unk.
//
// For files before the selected file it uses sectionLineCache actual heights where available,
// falling back to fileSectionLineCount estimates. This keeps the returned offset consistent
// with renderDiffPane (which also reads sectionLineCache), so the prewarm scrollAdjust loop
// sees matching lineOffset and scrollTop values and fires correctly.
func (m *model) computeUnkScrollOffset() int {
	files := m.visibleFiles()
	offset := 0
	for fi, f := range files {
		if fi == m.selectedFileIndex {
			offset++ // file header
			if !f.IsBinary && !f.IsTooLarge {
				switch m.layout.LayoutMode {
				case types.LayoutModeStack:
					offset += patch.LinesBeforeUnkStack(f.Patch, m.selectedUnkIndex, m.showUnkHeaders)
				case types.LayoutModeSplit:
					offset += patch.LinesBeforeUnkSplit(f.Patch, m.selectedUnkIndex, m.showUnkHeaders)
				default:
					offset += patch.LinesBeforeUnk(f.Patch, m.selectedUnkIndex)
				}
			}
			return offset
		}
		// Use actual cached height (consistent with renderDiffPane) so that
		// scrollTop is expressed in the same coordinate system as sectionLineCache.
		key := m.sectionCacheKey(f)
		if lc, ok := m.sectionLineCache[key]; ok {
			offset += lc
		} else {
			offset += m.sectionLineCountEstimate(f)
		}
	}
	return offset
}
