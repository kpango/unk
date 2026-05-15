package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/types"
)

// TestDiffSectionLineWidths_CJKFilePath verifies that a CJK file path in the
// section header row does not overflow DiffContentWidth.
// Root cause: the old rune-count truncation skips truncation when len(runes) < maxPathW
// even though visibleWidth(path) > maxPathW (e.g. 40 CJK runes = 80 cols, maxPathW=71).
func TestDiffSectionLineWidths_CJKFilePath(t *testing.T) {
	// 40 CJK chars = 80 terminal columns in a file path.
	cjkPath := strings.Repeat("あ", 40) + ".go"
	patch := "@@ -1,3 +1,3 @@\n-old\n+new\n ctx\n"
	f := goFile(cjkPath, patch, 1, 1)
	for _, termW := range []int{80, 120, 160, 220} {
		for _, tm := range testModes {
			label := fmt.Sprintf("termW=%d/%s CJK-file-path", termW, tm.name)
			m := makeWidthModel(termW, f, tm.mode, false)
			checkSectionLineWidths(t, m, f, label)
		}
	}
}

// TestDiffSectionLineWidths_CJKUnkHeader verifies that a CJK function name in the
// unk header context tail does not overflow DiffContentWidth.
// This is the bug scenario for Japanese/Chinese codebases where functions have
// non-ASCII names in the @@ ... @@ funcName() part.
func TestDiffSectionLineWidths_CJKUnkHeader(t *testing.T) {
	// Build a unk header with a CJK function name that exceeds typical widths.
	// 40 CJK chars = 80 terminal columns.
	cjkName := strings.Repeat("あ", 40) // 80 cols
	unkHeader := fmt.Sprintf("@@ -1,3 +1,3 @@ func %s(x int) error", cjkName)
	patch := fmt.Sprintf("%s\n-old line\n+new line\n ctx line\n", unkHeader)

	lang := "go"
	f := types.DiffFile{
		ID:       "cjk_unk.go",
		Path:     "cjk_unk.go",
		Patch:    patch,
		Language: &lang,
		Stats:    types.DiffStats{Additions: 1, Deletions: 1},
		Metadata: types.DiffMetadata{
			CacheKey: "ck-cjk-unk",
			Unks:     []types.DiffUnk{{Index: 0, Header: unkHeader}},
		},
	}

	t.Logf("unkHeader visible width: %d", lipgloss.Width(unkHeader))

	for _, termW := range []int{80, 120, 160, 220} {
		for _, tm := range testModes {
			label := fmt.Sprintf("termW=%d/%s CJK-unk-header", termW, tm.name)
			m := makeWidthModel(termW, f, tm.mode, false)
			checkSectionLineWidths(t, m, f, label)
		}
	}
}

// TestDiffSectionLineWidths_CJKSidebarFilename verifies CJK filenames in the
// sidebar do not cause the sidebar rows to overflow SidebarWidth.
// The sidebar is only rendered at termWidth >= 220 (ViewportFull).
func TestDiffSectionLineWidths_CJKSidebarFilename(t *testing.T) {
	cjkPath := strings.Repeat("あ", 30) + ".go" // CJK filename
	patch := "@@ -1,3 +1,3 @@\n-old\n+new\n ctx\n"
	f := goFile(cjkPath, patch, 1, 1)

	// Only test at termWidths where sidebar is rendered.
	for _, termW := range []int{220, 260} {
		label := fmt.Sprintf("termW=%d CJK-sidebar", termW)
		cs := types.Changeset{ID: "cjk-sb", Files: []types.DiffFile{f}}
		m := New(types.Bootstrap{Changeset: cs}).(*model)
		m.termWidth = termW
		m.termHeight = 50
		m.sidebarVisible = true
		m.layout = layout.ComputeLayout(m.termWidth, m.termHeight, types.LayoutModeSplit, 34, true, true)
		if !m.layout.RenderSidebar {
			t.Skipf("%s: sidebar not rendered (RenderSidebar=false)", label)
		}
		// Check sidebar rows are SidebarWidth wide.
		sidebarStr := m.renderSidebarInner()
		lines := strings.Split(sidebarStr, "\n")
		wantW := m.layout.SidebarWidth
		failures := 0
		for i, line := range lines {
			if line == "" {
				continue
			}
			got := terminalLineWidth(line)
			if got != wantW {
				t.Errorf("%s: sidebar line %d width=%d want=%d", label, i, got, wantW)
				failures++
				if failures >= 3 {
					break
				}
			}
		}
	}
}
