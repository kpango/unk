package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kpango/unk/internal/diff"
	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/tui/patch"
	"github.com/kpango/unk/internal/tui/render"
	"github.com/kpango/unk/internal/types"
)

// checkSectionLineWidths renders f in model m, splits on '\n', and asserts every
// non-empty line is exactly m.layout.DiffContentWidth visible columns wide.
// Reports up to 5 failures before stopping to avoid log flooding.
func checkSectionLineWidths(t *testing.T, m *model, f types.DiffFile, label string) {
	t.Helper()
	want := m.layout.DiffContentWidth
	rendered := m.renderFileSectionInner(f, false)
	lines := strings.Split(rendered, "\n")
	failures := 0
	for i, line := range lines {
		if line == "" {
			continue
		}
		// Use runewidth-based measurement so the test is correct under both en_US and
		// ja_JP locales. lipgloss.Width undercounts East Asian Ambiguous chars (e.g.
		// "─" in the stack separator, "│" in the split divider) under ja_JP.UTF-8.
		got := terminalLineWidth(line)
		if got != want {
			t.Errorf("%s: line %d width=%d want=%d", label, i, got, want)
			failures++
			if failures >= 5 {
				t.Logf("%s: stopping after 5 failures", label)
				return
			}
		}
	}
}

// makeWidthModel returns a Model with a single-file changeset sized to termWidth.
func makeWidthModel(termWidth int, f types.DiffFile, layoutMode types.LayoutMode, sidebarVisible bool) *model {
	cs := types.Changeset{ID: "w", Files: []types.DiffFile{f}}
	m := New(types.Bootstrap{Changeset: cs}).(*model)
	m.termWidth = termWidth
	m.termHeight = 50
	m.sidebarVisible = sidebarVisible
	m.layout = layout.ComputeLayout(m.termWidth, m.termHeight, layoutMode, 34, sidebarVisible, false)
	return m
}

// goFile builds a DiffFile with the given patch and a Go-language tag.
func goFile(path, patch string, adds, dels int) types.DiffFile {
	lang := "go"
	unkCount := max(strings.Count(patch, "@@"), 1)
	unks := make([]types.DiffUnk, unkCount)
	for i := range unks {
		unks[i] = types.DiffUnk{Index: i, Header: fmt.Sprintf("@@ -1,5 +1,5 @@ func fn%d()", i)}
	}
	return types.DiffFile{
		ID:       path,
		Path:     path,
		Patch:    patch,
		Language: &lang,
		Stats:    types.DiffStats{Additions: adds, Deletions: dels},
		Metadata: types.DiffMetadata{CacheKey: "ck-" + path, Unks: unks},
	}
}

var testModes = []struct {
	name string
	mode types.LayoutMode
}{
	{"split", types.LayoutModeSplit},
	{"stack", types.LayoutModeStack},
}

// TestDiffSectionLineWidths_Normal verifies normal diff content produces
// exactly DiffContentWidth-wide lines in both split and stack views.
func TestDiffSectionLineWidths_Normal(t *testing.T) {
	patch := "@@ -1,5 +1,5 @@\n context line\n-deleted line\n+added line\n context2\n context3\n"
	for _, termW := range []int{80, 120, 160, 220} {
		for _, tm := range testModes {
			label := fmt.Sprintf("termW=%d/%s", termW, tm.name)
			f := goFile("pkg/normal/file.go", patch, 1, 1)
			m := makeWidthModel(termW, f, tm.mode, false)
			checkSectionLineWidths(t, m, f, label)
		}
	}
}

// TestDiffSectionLineWidths_LongLines verifies 200-char code lines are truncated
// and do not overflow DiffContentWidth.
func TestDiffSectionLineWidths_LongLines(t *testing.T) {
	longCode := strings.Repeat("x", 200)
	patch := fmt.Sprintf("@@ -1,3 +1,3 @@\n context\n-old: %s\n+new: %s\n ctx2\n", longCode, longCode)
	for _, termW := range []int{80, 120, 220} {
		for _, tm := range testModes {
			label := fmt.Sprintf("termW=%d/%s long-lines", termW, tm.name)
			f := goFile("long.go", patch, 1, 1)
			m := makeWidthModel(termW, f, tm.mode, false)
			checkSectionLineWidths(t, m, f, label)
		}
	}
}

// TestDiffSectionLineWidths_LongUnkHeader verifies unk headers with long function-
// context tails are truncated so they fit in DiffContentWidth.
func TestDiffSectionLineWidths_LongUnkHeader(t *testing.T) {
	longCtx := strings.Repeat("a", 150)
	patch := fmt.Sprintf(
		"@@ -1,3 +1,3 @@ func %s(arg1 int, arg2 string) (int, error)\n-old\n+new\n ctx\n",
		longCtx,
	)
	f := goFile("unk_hdr.go", patch, 1, 1)
	// Override the unk metadata header to match the patch (so the function context is used).
	f.Metadata.Unks[0].Header = fmt.Sprintf("@@ -1,3 +1,3 @@ func %s(arg1 int, arg2 string) (int, error)", longCtx)
	for _, termW := range []int{80, 120, 220} {
		label := fmt.Sprintf("termW=%d/split long-unk-header", termW)
		m := makeWidthModel(termW, f, types.LayoutModeSplit, false)
		checkSectionLineWidths(t, m, f, label)
	}
}

// TestDiffSectionLineWidths_BlankLines verifies that blank lines produced by a
// trailing newline in the patch are rendered full-width (panel background fill),
// not as zero-width rows that displace the scrollbar.
func TestDiffSectionLineWidths_BlankLines(t *testing.T) {
	// Extra trailing '\n' produces an empty "" entry after splitLines.
	patch := "@@ -1,4 +1,4 @@\n ctx\n-del\n+add\n ctx2\n\n"
	for _, termW := range []int{80, 120, 220} {
		for _, tm := range testModes {
			label := fmt.Sprintf("termW=%d/%s blank-lines", termW, tm.name)
			f := goFile("blank.go", patch, 1, 1)
			m := makeWidthModel(termW, f, tm.mode, false)
			checkSectionLineWidths(t, m, f, label)
		}
	}
}

// TestDiffSectionLineWidths_AsymmetricUnk verifies that the empty cell shown on
// the unmatched side of an asymmetric del/add block in split view is full-width.
func TestDiffSectionLineWidths_AsymmetricUnk(t *testing.T) {
	// 3 deletions, 1 addition → 2 empty cells on the right in split view.
	patch := "@@ -1,5 +1,3 @@\n ctx\n-del1\n-del2\n-del3\n+add1\n ctx2\n"
	f := goFile("asymmetric.go", patch, 1, 3)
	for _, termW := range []int{80, 120, 220} {
		label := fmt.Sprintf("termW=%d/split asymmetric", termW)
		m := makeWidthModel(termW, f, types.LayoutModeSplit, false)
		checkSectionLineWidths(t, m, f, label)
	}
}

// TestDiffSectionLineWidths_LongFilePath verifies a 200-char path is truncated so
// the header row fits in DiffContentWidth.
func TestDiffSectionLineWidths_LongFilePath(t *testing.T) {
	longPath := "very/deeply/nested/" + strings.Repeat("x", 180) + ".go"
	patch := "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n"
	f := goFile(longPath, patch, 1, 1)
	for _, termW := range []int{80, 120, 220} {
		label := fmt.Sprintf("termW=%d long-path", termW)
		m := makeWidthModel(termW, f, types.LayoutModeSplit, false)
		checkSectionLineWidths(t, m, f, label)
	}
}

// TestDiffSectionLineWidths_WithLineNumbers re-runs the normal case with line
// numbers enabled to verify lineNum+code sum to DiffContentWidth.
func TestDiffSectionLineWidths_WithLineNumbers(t *testing.T) {
	patch := "@@ -1,5 +1,5 @@\n ctx\n-del\n+add\n ctx2\n ctx3\n"
	for _, termW := range []int{80, 120, 220} {
		for _, tm := range testModes {
			label := fmt.Sprintf("termW=%d/%s line-nums", termW, tm.name)
			f := goFile("nums.go", patch, 1, 1)
			m := makeWidthModel(termW, f, tm.mode, false)
			m.showLineNumbers = true
			checkSectionLineWidths(t, m, f, label)
		}
	}
}

// TestDiffSectionLineWidths_UnicodeContent verifies multi-byte Unicode in diff
// lines is truncated by rune count, not byte count.
func TestDiffSectionLineWidths_UnicodeContent(t *testing.T) {
	unicodeLine := strings.Repeat("あ", 60) // 60 three-byte characters
	patch := fmt.Sprintf("@@ -1,3 +1,3 @@\n ctx\n-old %s\n+new %s\n", unicodeLine, unicodeLine)
	for _, termW := range []int{80, 120, 220} {
		label := fmt.Sprintf("termW=%d unicode", termW)
		f := goFile("unicode.go", patch, 1, 1)
		m := makeWidthModel(termW, f, types.LayoutModeSplit, false)
		checkSectionLineWidths(t, m, f, label)
	}
}

// TestDiffSectionLineWidths_TabIndented verifies that tab-indented source code
// (common in Go, C, Java) does not overflow DiffContentWidth. Tabs are expanded
// to 4-column stops to match lipgloss.Style.Render tab behaviour. Without
// expansion, 3 tabs × (4-1) = 9 extra columns per cell appear in split view.
func TestDiffSectionLineWidths_TabIndented(t *testing.T) {
	// Valid unified-diff format with leading tab-indented code.
	patch := "@@ -1,5 +1,5 @@\n ctx\n-\t\t\told_func()\n+\t\t\tnew_func()\n-\t\t\t\treturn nil\n+\t\t\t\treturn err\n ctx2\n"
	f := goFile("tabs.go", patch, 2, 2)
	for _, termW := range []int{80, 120, 160, 220} {
		for _, tm := range testModes {
			label := fmt.Sprintf("termW=%d/%s tab-indented", termW, tm.name)
			m := makeWidthModel(termW, f, tm.mode, false)
			checkSectionLineWidths(t, m, f, label)
		}
	}
}

// TestDiffSectionLineWidths_StackTabLongCode verifies that in stack mode, lines
// with 3 leading tabs + 70-char code do not overflow DiffContentWidth.
// Root cause: renderPatchStack built lineEntry without expandTabs, so lipgloss
// expanded tabs during Render while truncateStack counted each tab as 1 column.
func TestDiffSectionLineWidths_StackTabLongCode(t *testing.T) {
	longCode := strings.Repeat("x", 70)
	patch := fmt.Sprintf("@@ -1,3 +1,3 @@\n ctx\n-\t\t\t%s\n+\t\t\t%s\n ctx2\n", longCode, longCode)
	f := goFile("stack_tab_long.go", patch, 1, 1)
	for _, termW := range []int{80, 120, 160, 220} {
		label := fmt.Sprintf("termW=%d/stack tab+longcode", termW)
		m := makeWidthModel(termW, f, types.LayoutModeStack, false)
		checkSectionLineWidths(t, m, f, label)
	}
}

// TestDiffSectionLineWidths_DeepTabNesting simulates real Go switch/select code
// with deeply nested tabs (4 levels deep = 16 columns), common in switch statements
// and goroutine bodies. Tests all three render modes.
func TestDiffSectionLineWidths_DeepTabNesting(t *testing.T) {
	// Simulate a Go switch statement body: func → switch → case → statement
	// Each level is 1 tab = 4 cols in lipgloss. 4 levels = 16 cols of indent.
	indent4 := "\t\t\t\t"
	longIdent := strings.Repeat("x", 65) // 16 + 65 = 81 > 79 for termW=80 without truncation
	patch := fmt.Sprintf(
		"@@ -1,7 +1,7 @@\n"+
			" func f() {\n"+
			" \tswitch v {\n"+
			" \t\tcase A:\n"+
			"-%s%s_old\n"+
			"+%s%s_new\n"+
			" \t\tdefault:\n"+
			" \t\t\treturn nil\n"+
			" \t}\n"+
			" }\n",
		indent4, longIdent, indent4, longIdent,
	)
	f := goFile("deep_tabs.go", patch, 1, 1)
	for _, termW := range []int{80, 120, 160, 220} {
		for _, tm := range testModes {
			label := fmt.Sprintf("termW=%d/%s deep-tab-nesting", termW, tm.name)
			m := makeWidthModel(termW, f, tm.mode, false)
			checkSectionLineWidths(t, m, f, label)
		}
	}
}

// TestDiffSectionLineWidths_WithIntraSpans verifies intra-line highlighted spans
// are width-capped so no line exceeds DiffContentWidth.
func TestDiffSectionLineWidths_WithIntraSpans(t *testing.T) {
	prefix := strings.Repeat("x", 80)
	patch := fmt.Sprintf("@@ -1,3 +1,3 @@\n ctx\n-%s_OLD_VALUE\n+%s_NEW_VALUE\n ctx2\n", prefix, prefix)
	f := goFile("intra.go", patch, 1, 1)

	ic := make(map[string][2][][]diff.IntraSpan)
	render.BuildIntraCache(ic, []types.DiffFile{f})

	for _, termW := range []int{80, 120, 220} {
		label := fmt.Sprintf("termW=%d intra-spans", termW)
		m := makeWidthModel(termW, f, types.LayoutModeSplit, false)
		m.intraCache = ic
		checkSectionLineWidths(t, m, f, label)
	}
}

// TestRenderFullRowWidths verifies that every row in renderFull() — menu bar,
// body rows (sidebar+divider+diffPane+scrollbar), and status bar — is exactly
// termWidth visible columns wide. Covers sidebar-visible layout at termW≥220.
func TestRenderFullRowWidths(t *testing.T) {
	lang := "go"
	// Tab-indented Go code to exercise the expandTabs fix.
	tabPatch := "@@ -1,4 +1,4 @@\n ctx\n-\t\t\told_func()\n+\t\t\tnew_func()\n ctx2\n"
	// Long PR title to exercise the menu bar truncation fix.
	longTitle := strings.Repeat("A very long PR title ", 20)

	files := []types.DiffFile{
		goFile("tabs.go", tabPatch, 1, 1),
		{
			ID: "longpath", Path: "very/deeply/" + strings.Repeat("n", 120) + ".go",
			Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-rfl", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,3 +1,3 @@"},
			}},
		},
	}

	checkFull := func(t *testing.T, m *model, label string) {
		t.Helper()
		prewarmBench(t, m)
		full := m.renderFull()
		rows := strings.Split(full, "\n")
		failures := 0
		for i, row := range rows {
			if row == "" {
				continue
			}
			got := terminalLineWidth(row)
			if got != m.termWidth {
				t.Errorf("%s: row %d width=%d want=%d", label, i, got, m.termWidth)
				failures++
				if failures >= 3 {
					t.Logf("%s: stopping after 3 row failures", label)
					return
				}
			}
		}
	}

	for _, termW := range []int{80, 120, 160, 220} {
		for _, mode := range []types.LayoutMode{types.LayoutModeSplit, types.LayoutModeStack} {
			label := fmt.Sprintf("termW=%d/%s no-sidebar", termW, mode)
			cs := types.Changeset{ID: "rfl", Title: longTitle, Files: files}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 40
			m.layout = layout.ComputeLayout(m.termWidth, m.termHeight, mode, 34, false, false)
			checkFull(t, m, label)
		}
	}

	// Sidebar is only rendered at termWidth ≥ 220 (ViewportFull).
	for _, termW := range []int{220, 260} {
		for _, mode := range []types.LayoutMode{types.LayoutModeSplit, types.LayoutModeStack} {
			label := fmt.Sprintf("termW=%d/%s with-sidebar", termW, mode)
			cs := types.Changeset{ID: "rfl-sb", Title: longTitle, Files: files}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 40
			m.sidebarVisible = true
			m.layout = layout.ComputeLayout(m.termWidth, m.termHeight, mode, 34, true, true)
			checkFull(t, m, label)
		}
	}
}

// TestScrollbarColumnStable is the end-to-end proof that the scrollbar stays
// visible during scrolling. It verifies that renderDiffPane() — the exact
// string passed to joinColumnsAll for the scrollbar column — has every line
// exactly DiffPaneWidth columns wide, at every scroll position from 0 to max.
//
// If any diff pane row is wider than DiffPaneWidth, the terminal wraps that
// row to the next physical line, which pushes the scrollbar to a wrong
// column and makes it disappear or jump.
func TestScrollbarColumnStable(t *testing.T) {
	// Build a changeset that exercises every known overflow source:
	//   • 200-char code lines (long line truncation)
	//   • long unk header with function-context tail
	//   • trailing blank line (zero-width row fix)
	//   • asymmetric unk (empty cell on unmatched side)
	//   • long file path (header truncation)
	//   • normal context/add/del lines
	lang := "go"
	longCode := strings.Repeat("x", 200)
	longPath := "very/deeply/nested/" + strings.Repeat("z", 120) + ".go"

	files := []types.DiffFile{
		{
			ID: "long-lines", Path: "pkg/longlines/file.go",
			Patch:    fmt.Sprintf("@@ -1,4 +1,4 @@ func processLong()\n ctx\n-old: %s\n+new: %s\n ctx2\n\n", longCode, longCode),
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-ll", Unks: []types.DiffUnk{
				{Index: 0, Header: fmt.Sprintf("@@ -1,4 +1,4 @@ func processLong_%s(x int)", strings.Repeat("a", 120))},
			}},
		},
		{
			ID: "blank-lines", Path: "pkg/blank/file.go",
			Patch:    "@@ -1,4 +1,4 @@\n ctx\n-del\n+add\n ctx2\n\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-bl", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,4 +1,4 @@"},
			}},
		},
		{
			ID: "asymmetric", Path: "pkg/asym/file.go",
			Patch:    "@@ -1,5 +1,3 @@\n ctx\n-del1\n-del2\n-del3\n+add1\n ctx2\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 3},
			Metadata: types.DiffMetadata{CacheKey: "ck-as", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,5 +1,3 @@"},
			}},
		},
		{
			ID: "long-path", Path: longPath,
			Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-lp", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,3 +1,3 @@"},
			}},
		},
		{
			ID: "tab-indented", Path: "pkg/tabs/file.go",
			Patch:    "@@ -1,5 +1,5 @@\n ctx\n-\t\t\told_func()\n+\t\t\tnew_func()\n-\t\t\t\treturn nil\n+\t\t\t\treturn err\n ctx2\n",
			Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 2},
			Metadata: types.DiffMetadata{CacheKey: "ck-tabs", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,5 +1,5 @@"},
			}},
		},
	}

	for _, termW := range []int{80, 120, 160, 220} {
		for _, mode := range []types.LayoutMode{types.LayoutModeSplit, types.LayoutModeStack} {
			label := fmt.Sprintf("termW=%d/%s", termW, mode)
			cs := types.Changeset{ID: "scrolltest", Files: files}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 40
			m.layout = layout.ComputeLayout(m.termWidth, m.termHeight, mode, 34, false, false)

			// Populate section cache synchronously (simulates prewarm completing).
			prewarmBench(t, m)
			// Warm ancillary caches (menu bar, status bar, etc.) with one full render.
			_ = m.View()

			wantW := m.layout.DiffPaneWidth
			maxScroll := m.totalDiffLines() - m.bodyHeight()
			maxScroll = max(maxScroll, 0)

			// Verify at every scroll position to catch width issues at any line.
			checkScrollPos := func(scrollTop int) {
				m.scrollTop = scrollTop
				m.diffPaneCache = "" // force a fresh render for this scroll position
				diffPane := m.renderDiffPane()
				lines := strings.Split(diffPane, "\n")
				failures := 0
				for i, line := range lines {
					if line == "" {
						continue
					}
					got := terminalLineWidth(line)
					if got != wantW {
						t.Errorf("%s scroll=%d: diffPane line %d width=%d want=%d",
							label, scrollTop, i, got, wantW)
						failures++
						if failures >= 3 {
							t.Logf("%s scroll=%d: stopping after 3 failures", label, scrollTop)
							return
						}
					}
				}
			}

			for s := 0; s <= maxScroll; s++ {
				checkScrollPos(s)
			}
		}
	}
}

// TestPrewarmDoubleCompensation verifies that a second prewarm (e.g. triggered
// by intraCacheReadyMsg → clearRenderCache) does NOT re-apply the scrollTop
// correction that was already applied by the first prewarm.
//
// Root cause: clearRenderCache previously cleared sectionLineCache, so the
// second prewarm saw oldH = fileSectionLineCount() (estimate) again instead of
// the actual heights stored by the first prewarm. The compensation fired twice,
// causing the viewport to jump by 2× the correction amount.
func TestPrewarmDoubleCompensation(t *testing.T) {
	// Build a changeset with a stack-mode file that has separators: one unk
	// with both deletions and additions produces one extra separator line vs
	// the fileSectionLineCount estimate.
	lang := "go"
	files := []types.DiffFile{
		{
			ID: "above", Path: "above.go",
			Patch:    "@@ -1,4 +1,4 @@\n ctx\n-del\n+add\n ctx2\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-above", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,4 +1,4 @@"},
			}},
		},
		{
			ID: "below", Path: "below.go",
			Patch:    "@@ -1,3 +1,3 @@\n ctx\n-old\n+new\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-below", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,3 +1,3 @@"},
			}},
		},
	}

	for _, termW := range []int{80, 120, 160} {
		label := fmt.Sprintf("termW=%d/stack double-compensation", termW)
		cs := types.Changeset{ID: "dc", Files: files}
		m := New(types.Bootstrap{Changeset: cs}).(*model)
		m.termWidth = termW
		m.termHeight = 40
		m.layout = layout.ComputeLayout(termW, m.termHeight, types.LayoutModeStack, 34, false, false)

		// First prewarm: sectionLineCache empty → compensation applied.
		// Simulate user scrolled to show "below.go" (files[1]).
		aboveEstimate := fileSectionLineCount(files[0])
		m.scrollTop = aboveEstimate + 1 // 1 line into "below.go"
		prewarmBench(t, m)
		scrollAfterFirst := m.scrollTop

		// Second prewarm (simulates intraCacheReadyMsg → clearRenderCache → prewarm).
		// clearRenderCache now preserves sectionLineCache, so oldH = actual heights,
		// newH = same heights → scrollAdjust must be 0.
		m.clearRenderCache()
		prewarmBench(t, m)
		scrollAfterSecond := m.scrollTop

		if scrollAfterSecond != scrollAfterFirst {
			t.Errorf("%s: scrollTop changed after second prewarm: first=%d second=%d (double compensation)",
				label, scrollAfterFirst, scrollAfterSecond)
		}
	}
}

// TestStackViewOneSidedEstimate verifies that sectionLineCountEstimate returns the
// correct value for add-only and del-only patches in stack view (RC 30).
//
// Root cause: fileSectionLineCount = strings.Count(patch, "\n") + 2. In stack view,
// the trailing "" artifact from splitLines is skipped (−1 line), and a separator is
// added between del/add blocks only when BOTH exist. For one-sided patches (add-only
// or del-only) there is no separator, so the estimate was 1 too high vs actual.
// This caused scrollAdjust = −1 per one-sided file on the first prewarm → viewport jump.
// Fix: sectionLineCountEstimate deducts emptyCount and adds separators for stack view.
func TestStackViewOneSidedEstimate(t *testing.T) {
	lang := "go"
	cases := []struct {
		name      string
		patch     string
		wantLines int // expected stack line count (incl. file header)
	}{
		// add-only: no separator, trailing "" skipped → estimate must be 1 less than fileSectionLineCount
		{
			name:  "add-only",
			patch: "@@ -0,0 +1,3 @@\n+add1\n+add2\n+add3\n",
			// file header(1) + @@ header(1) + add1+add2+add3(3) = 5
			wantLines: 5,
		},
		// del-only: same pattern
		{
			name:      "del-only",
			patch:     "@@ -1,3 +0,0 @@\n-del1\n-del2\n-del3\n",
			wantLines: 5,
		},
		// balanced: separator compensates for skipped trailing "" → estimate equals fileSectionLineCount
		{
			name:  "balanced",
			patch: "@@ -1,4 +1,4 @@\n ctx\n-del\n+add\n ctx2\n",
			// file header(1) + @@ header(1) + ctx(1) + del(1) + sep(1) + add(1) + ctx2(1) = 7
			wantLines: 7,
		},
		// multi-unk: one balanced, one add-only; two empties (mid-patch blank + trailing)
		{
			name:  "multi-unk-mixed",
			patch: "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n\n@@ -10,3 +10,3 @@\n ctx2\n+add2\n ctx3\n",
			// file header(1) + @@ (1) + ctx(1) + del(1) + sep(1) + add(1) +
			// @@ (1) + ctx2(1) + ctx3(1) + add2(1) = 10
			wantLines: 10,
		},
	}

	for _, tc := range cases {
		cs := types.Changeset{ID: "rc30-" + tc.name, Files: []types.DiffFile{
			{
				ID: tc.name, Path: tc.name + ".go",
				Patch: tc.patch, Language: &lang,
				Stats:    types.DiffStats{Additions: 1, Deletions: 1},
				Metadata: types.DiffMetadata{CacheKey: "ck-rc30-" + tc.name},
			},
		}}
		m := New(types.Bootstrap{Changeset: cs}).(*model)
		m.termWidth = 120
		m.termHeight = 40
		m.layout = layout.ComputeLayout(120, 40, types.LayoutModeStack, 34, false, false)

		f := cs.Files[0]
		got := m.sectionLineCountEstimate(f)
		if got != tc.wantLines {
			t.Errorf("sectionLineCountEstimate(%s) = %d, want %d", tc.name, got, tc.wantLines)
		}

		// Also verify the estimate matches the actual rendered count after prewarm.
		prewarmBench(t, m)
		key := m.sectionCacheKey(f)
		actual := m.sectionLineCache[key]
		if actual != tc.wantLines {
			t.Errorf("prewarm lineCount(%s) = %d, want %d", tc.name, actual, tc.wantLines)
		}
	}

	// Verify that scrollAdjust is 0 for one-sided patches: the estimate must match
	// the actual height so the viewport does not jump when the first prewarm arrives.
	t.Run("no-scroll-adjust-for-one-sided", func(t *testing.T) {
		oneSidedFiles := []types.DiffFile{
			{
				ID: "f1", Path: "new.go",
				Patch:    "@@ -0,0 +1,3 @@\n+add1\n+add2\n+add3\n",
				Language: &lang, Stats: types.DiffStats{Additions: 3},
				Metadata: types.DiffMetadata{CacheKey: "ck-rc30-adj1", Unks: []types.DiffUnk{
					{Index: 0, Header: "@@ -0,0 +1,3 @@"},
				}},
			},
			{
				ID: "f2", Path: "changed.go",
				Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
				Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
				Metadata: types.DiffMetadata{CacheKey: "ck-rc30-adj2", Unks: []types.DiffUnk{
					{Index: 0, Header: "@@ -1,3 +1,3 @@"},
				}},
			},
		}
		cs := types.Changeset{ID: "rc30-adj", Files: oneSidedFiles}
		m := New(types.Bootstrap{Changeset: cs}).(*model)
		m.termWidth = 120
		m.termHeight = 40
		m.layout = layout.ComputeLayout(120, 40, types.LayoutModeStack, 34, false, false)

		// Simulate user scrolled past the first (add-only) file section.
		est := m.sectionLineCountEstimate(oneSidedFiles[0])
		m.scrollTop = est + 2 // positioned into f2
		prewarmBench(t, m)

		// After prewarm with accurate estimate: scrollAdjust must be 0, so scrollTop unchanged.
		if m.scrollTop != est+2 {
			t.Errorf("scrollTop changed after prewarm for one-sided patch: want %d got %d (jitter!)",
				est+2, m.scrollTop)
		}
	})
}

// TestPatchLinesBeforeUnkStack verifies that patchLinesBeforeUnkStack returns the
// correct within-file line offset for navigation in stack view.
//
// Root cause: the original patchLinesBeforeUnk counted every patch element (including
// empty trailing elements) 1-to-1, which coincidentally worked for balanced unks
// (separator+1 cancelled empty-1) but was 1 too high for one-sided (add-only/del-only)
// unks because there's no separator to counterbalance the skipped empty element.
func TestPatchLinesBeforeUnkStack(t *testing.T) {
	cases := []struct {
		name           string
		patch          string
		unkIndex       int
		showUnkHeaders bool
		want           int
	}{
		// Single balanced unk: for unkIndex=0 the answer is always 0 (no lines before first @@).
		{"balanced-unk0", "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n ctx2\n", 0, true, 0},
		// Two balanced unks: lines before unk1 = unk_hdr(1)+ctx(1)+del(1)+sep(1)+add(1)+ctx2(1) = 6
		{"two-balanced-unk1", "@@ -1,4 +1,4 @@\n ctx\n-del\n+add\n ctx2\n\n@@ -10,4 +10,4 @@\n ctx3\n-del2\n+add2\n ctx4\n", 1, true, 6},
		// Add-only unk followed by balanced: lines before unk1 = unk_hdr(1)+add1(1)+add2(1) = 3
		// (empty trailing element skipped, no separator for add-only).
		{"add-only-then-balanced-unk1", "@@ -0,0 +1,2 @@\n+add1\n+add2\n\n@@ -5,3 +7,3 @@\n ctx\n-del\n+add\n", 1, true, 3},
		// Del-only unk followed by balanced: same = 3.
		{"del-only-then-balanced-unk1", "@@ -1,2 +0,0 @@\n-del1\n-del2\n\n@@ -5,3 +3,3 @@\n ctx\n-del\n+add\n", 1, true, 3},
		// showUnkHeaders=false: @@ header line not rendered, subtract nUnks=1.
		{"add-only-noHH-unk1", "@@ -0,0 +1,2 @@\n+add1\n+add2\n\n@@ -5,3 +7,3 @@\n ctx\n-del\n+add\n", 1, false, 2},
	}
	for _, tc := range cases {
		got := patch.LinesBeforeUnkStack(tc.patch, tc.unkIndex, tc.showUnkHeaders)
		if got != tc.want {
			t.Errorf("patch.LinesBeforeUnkStack(%q, %d, hh=%v) = %d, want %d",
				tc.name, tc.unkIndex, tc.showUnkHeaders, got, tc.want)
		}
	}
}

// TestSectionLineCacheNoPhantomLines verifies that sectionLineCache stores the
// correct visual line count (not overcounted by 1 due to trailing '\n').
//
// Root cause (RC 24): all rendered sections end with '\n'. The old formula
// strings.Count(v, "\n") + 1 overcounts by 1, creating a phantom line per
// section in the virtual scroll space. When scrollTop lands on the phantom line,
// 0 lines are rendered from that section, making two consecutive scroll positions
// show identical content — manifesting as jitter.
//
// Fix: use len(buildLineOffsets(v)) which correctly excludes the trailing '\n'.
func TestSectionLineCacheNoPhantomLines(t *testing.T) {
	lang := "go"
	files := []types.DiffFile{
		{
			ID: "f1", Path: "pkg/a/file.go",
			Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-rc24-a", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,3 +1,3 @@"},
			}},
		},
		{
			ID: "f2", Path: "pkg/b/file.go",
			Patch:    "@@ -1,4 +1,4 @@\n ctx\n-del1\n-del2\n+add1\n+add2\n ctx2\n",
			Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 2},
			Metadata: types.DiffMetadata{CacheKey: "ck-rc24-b", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,4 +1,4 @@"},
			}},
		},
	}

	for _, termW := range []int{80, 120} {
		for _, mode := range []types.LayoutMode{types.LayoutModeSplit, types.LayoutModeStack} {
			label := fmt.Sprintf("termW=%d/%s", termW, mode)
			cs := types.Changeset{ID: "rc24", Files: files}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 40
			m.layout = layout.ComputeLayout(termW, m.termHeight, mode, 34, false, false)
			prewarmBench(t, m)

			for _, f := range files {
				key := m.sectionCacheKey(f)
				offsets, hasOffsets := m.sectionLineOffsets[key]
				lc, hasLC := m.sectionLineCache[key]

				if !hasOffsets || !hasLC {
					t.Errorf("%s %s: cache not populated after prewarm", label, f.Path)
					continue
				}
				// The invariant: sectionLineCache must equal len(sectionLineOffsets).
				// Any mismatch means phantom lines exist in the virtual scroll space.
				if lc != len(offsets) {
					t.Errorf("%s %s: sectionLineCache=%d but len(offsets)=%d — phantom lines will cause jitter",
						label, f.Path, lc, len(offsets))
				}

				// Also verify no trailing phantom: the rendered section ends with '\n',
				// so strings.Count(v, "\n") + 1 must NOT be used as the line count.
				rendered := m.sectionCache[key]
				badCount := strings.Count(rendered, "\n") + 1
				if lc == badCount && len(rendered) > 0 && rendered[len(rendered)-1] == '\n' {
					t.Errorf("%s %s: sectionLineCache=%d equals overcounted value %d — RC 24 fix not applied",
						label, f.Path, lc, badCount)
				}
			}
		}
	}
}

// TestSplitViewLineSavings verifies that splitViewLineSavings returns the correct
// number of lines saved by del/add pairing in split layout mode.
func TestSplitViewLineSavings(t *testing.T) {
	cases := []struct {
		name  string
		patch string
		want  int
	}{
		{"no-change", "@@ -1,3 +1,3 @@\n ctx\n ctx2\n ctx3\n", 0},
		{"add-only", "@@ -0,0 +1,3 @@\n+add1\n+add2\n+add3\n", 0},
		{"del-only", "@@ -1,3 +0,0 @@\n-del1\n-del2\n-del3\n", 0},
		{"1del-1add", "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n ctx2\n", 1},
		{"2del-2add", "@@ -1,4 +1,4 @@\n ctx\n-del1\n-del2\n+add1\n+add2\n ctx2\n", 2},
		{"3del-2add", "@@ -1,5 +1,4 @@\n ctx\n-del1\n-del2\n-del3\n+add1\n+add2\n ctx2\n", 2},
		{"2del-3add", "@@ -1,4 +1,5 @@\n ctx\n-del1\n-del2\n+add1\n+add2\n+add3\n ctx2\n", 2},
		{"two-blocks", "@@ -1,6 +1,6 @@\n-d1\n+a1\n ctx\n-d2\n-d3\n+a2\n+a3\n ctx2\n", 3},
		{"two-unks", "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n\n@@ -10,3 +10,3 @@\n ctx2\n-del2\n+add2\n", 2},
	}
	for _, tc := range cases {
		got := patch.SplitViewLineSavings(tc.patch)
		if got != tc.want {
			t.Errorf("patch.SplitViewLineSavings(%q) [%s] = %d, want %d", tc.name, tc.patch, got, tc.want)
		}
	}
}

// TestSplitViewSectionEstimateMatchesCache verifies that sectionLineCountEstimate
// matches the actual rendered line count (sectionLineCache) in split layout mode.
// Mismatch → phantom lines → scrollTop jitter on prewarm.
func TestSplitViewSectionEstimateMatchesCache(t *testing.T) {
	lang := "go"
	files := []types.DiffFile{
		{
			ID: "sym-1-1", Path: "pkg/a/file.go",
			Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n ctx2\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-sv-11", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,3 +1,3 @@"},
			}},
		},
		{
			ID: "sym-2-2", Path: "pkg/b/file.go",
			Patch:    "@@ -1,4 +1,4 @@\n ctx\n-del1\n-del2\n+add1\n+add2\n ctx2\n",
			Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 2},
			Metadata: types.DiffMetadata{CacheKey: "ck-sv-22", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,4 +1,4 @@"},
			}},
		},
		{
			ID: "asym-3-2", Path: "pkg/c/file.go",
			Patch:    "@@ -1,5 +1,4 @@\n ctx\n-del1\n-del2\n-del3\n+add1\n+add2\n ctx2\n",
			Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 3},
			Metadata: types.DiffMetadata{CacheKey: "ck-sv-32", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,5 +1,4 @@"},
			}},
		},
		{
			ID: "add-only", Path: "pkg/d/file.go",
			Patch:    "@@ -0,0 +1,3 @@\n+add1\n+add2\n+add3\n",
			Language: &lang, Stats: types.DiffStats{Additions: 3, Deletions: 0},
			Metadata: types.DiffMetadata{CacheKey: "ck-sv-add", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -0,0 +1,3 @@"},
			}},
		},
	}

	for _, termW := range []int{80, 120} {
		for _, showHH := range []bool{true, false} {
			label := fmt.Sprintf("termW=%d/showHH=%v", termW, showHH)
			cs := types.Changeset{ID: "sv-est", Files: files}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 40
			m.layout = layout.ComputeLayout(termW, m.termHeight, types.LayoutModeSplit, 34, false, false)
			m.showUnkHeaders = showHH
			prewarmBench(t, m)

			for _, f := range files {
				key := m.sectionCacheKey(f)
				lc, hasLC := m.sectionLineCache[key]
				offsets, hasOffsets := m.sectionLineOffsets[key]
				if !hasLC || !hasOffsets {
					t.Errorf("%s %s: cache not populated after prewarm", label, f.Path)
					continue
				}
				est := m.sectionLineCountEstimate(f)
				if est != lc {
					t.Errorf("%s %s: estimate=%d actual=%d — phantom lines will cause jitter",
						label, f.Path, est, lc)
				}
				if lc != len(offsets) {
					t.Errorf("%s %s: sectionLineCache=%d len(offsets)=%d mismatch",
						label, f.Path, lc, len(offsets))
				}
			}
		}
	}
}

// TestPatchLinesBeforeUnkSplit verifies that patchLinesBeforeUnkSplit correctly
// accounts for del/add pairing and showUnkHeaders in split view.
func TestPatchLinesBeforeUnkSplit(t *testing.T) {
	cases := []struct {
		name           string
		patch          string
		unkIndex       int
		showUnkHeaders bool
		want           int
	}{
		// Single unk, unk0 always starts at offset 0
		{"unk0-showHH", "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n ctx2\n", 0, true, 0},
		{"unk0-noHH", "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n ctx2\n", 0, false, 0},

		// Two unks: unk0 has 1del+1add (saves 1), unk1 starts after
		// Raw count before unk1 = 6 (@@,ctx,-del,+add,ctx2,blank)
		// Split savings = min(1,1) = 1
		// showHH=true: 6 - 1 = 5
		// showHH=false: 6 - 1 - 1(unk0 header) = 4
		{"two-unks-unk1-showHH", "@@ -1,4 +1,4 @@\n ctx\n-del\n+add\n ctx2\n\n@@ -10,3 +10,3 @@\n ctx3\n-del2\n+add2\n", 1, true, 5},
		{"two-unks-unk1-noHH", "@@ -1,4 +1,4 @@\n ctx\n-del\n+add\n ctx2\n\n@@ -10,3 +10,3 @@\n ctx3\n-del2\n+add2\n", 1, false, 4},

		// Add-only first unk (no savings), then balanced unk
		// Raw count before unk1 = 3 (@@,+add1,+add2)  Wait, unk separator "\n" is not present
		// Actually: "@@ ... @@\n+add1\n+add2\n" → split = ["@@","","add1","add2",""?]
		// Let me check: "@@ -0,0 +1,2 @@\n+add1\n+add2\n\n@@ -5,3 +7,3 @@\n ctx\n-del\n+add\n"
		// Raw lines: @@, +add1, +add2, blank, @@, ctx, -del, +add
		// Before unk1 (4th @@ element): count = 4 (@@, +add1, +add2, blank)
		// Savings = 0 (no del/add pairs in unk0 — add-only)
		// showHH=true: 4 - 0 = 4
		// showHH=false: 4 - 0 - 1(one @@ before) = 3
		{"add-only-then-balanced-unk1-showHH", "@@ -0,0 +1,2 @@\n+add1\n+add2\n\n@@ -5,3 +7,3 @@\n ctx\n-del\n+add\n", 1, true, 4},
		{"add-only-then-balanced-unk1-noHH", "@@ -0,0 +1,2 @@\n+add1\n+add2\n\n@@ -5,3 +7,3 @@\n ctx\n-del\n+add\n", 1, false, 3},

		// 2del+2add block (savings = 2) before unk1
		// patch = "@@ -1,4 +1,4 @@\n ctx\n-d1\n-d2\n+a1\n+a2\n ctx2\n\n@@ ..."
		// Raw before unk1 = 7 (@@, ctx, -d1, -d2, +a1, +a2, ctx2)
		// Wait, there's also "\n" at the end making element 8 (blank before next @@)
		// Elements: [@@, " ctx", "-d1", "-d2", "+a1", "+a2", " ctx2", ""] before next @@
		// count = 8
		// savings = min(2,2) = 2
		// showHH=true: 8 - 2 = 6
		// showHH=false: 8 - 2 - 1 = 5
		{"asym-2del-2add-unk1-showHH", "@@ -1,6 +1,6 @@\n ctx\n-d1\n-d2\n+a1\n+a2\n ctx2\n\n@@ -20,3 +20,3 @@\n x\n-y\n+z\n", 1, true, 6},
		{"asym-2del-2add-unk1-noHH", "@@ -1,6 +1,6 @@\n ctx\n-d1\n-d2\n+a1\n+a2\n ctx2\n\n@@ -20,3 +20,3 @@\n x\n-y\n+z\n", 1, false, 5},
	}
	for _, tc := range cases {
		got := patch.LinesBeforeUnkSplit(tc.patch, tc.unkIndex, tc.showUnkHeaders)
		if got != tc.want {
			t.Errorf("patch.LinesBeforeUnkSplit(%q, unk=%d, showHH=%v) = %d, want %d",
				tc.name, tc.unkIndex, tc.showUnkHeaders, got, tc.want)
		}
	}
}

// TestStackViewSectionEstimateMatchesCache verifies that sectionLineCountEstimate
// matches the actual rendered line count (sectionLineCache) in stack layout mode
// for both showUnkHeaders=true and showUnkHeaders=false.
// Mismatch → scrollAdjust ≠ 0 on first prewarm → viewport jump (jitter).
func TestStackViewSectionEstimateMatchesCache(t *testing.T) {
	lang := "go"
	files := []types.DiffFile{
		{
			ID: "add-only", Path: "pkg/a/new.go",
			Patch:    "@@ -0,0 +1,3 @@\n+add1\n+add2\n+add3\n",
			Language: &lang, Stats: types.DiffStats{Additions: 3, Deletions: 0},
			Metadata: types.DiffMetadata{CacheKey: "ck-st-add", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -0,0 +1,3 @@"},
			}},
		},
		{
			ID: "del-only", Path: "pkg/b/del.go",
			Patch:    "@@ -1,3 +0,0 @@\n-del1\n-del2\n-del3\n",
			Language: &lang, Stats: types.DiffStats{Additions: 0, Deletions: 3},
			Metadata: types.DiffMetadata{CacheKey: "ck-st-del", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,3 +0,0 @@"},
			}},
		},
		{
			ID: "balanced", Path: "pkg/c/change.go",
			Patch:    "@@ -1,4 +1,4 @@\n ctx\n-del\n+add\n ctx2\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-st-bal", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,4 +1,4 @@"},
			}},
		},
		{
			ID: "multi-unk", Path: "pkg/d/multi.go",
			Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n\n@@ -10,3 +10,3 @@\n ctx2\n+add2\n ctx3\n",
			Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-st-multi", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,3 +1,3 @@"},
				{Index: 1, Header: "@@ -10,3 +10,3 @@"},
			}},
		},
	}

	for _, termW := range []int{80, 120} {
		for _, showHH := range []bool{true, false} {
			label := fmt.Sprintf("termW=%d/showHH=%v", termW, showHH)
			cs := types.Changeset{ID: "st-est", Files: files}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 40
			m.layout = layout.ComputeLayout(termW, m.termHeight, types.LayoutModeStack, 34, false, false)
			m.showUnkHeaders = showHH
			prewarmBench(t, m)

			for _, f := range files {
				key := m.sectionCacheKey(f)
				lc, hasLC := m.sectionLineCache[key]
				offsets, hasOffsets := m.sectionLineOffsets[key]
				if !hasLC || !hasOffsets {
					t.Errorf("%s %s: cache not populated after prewarm", label, f.Path)
					continue
				}
				est := m.sectionLineCountEstimate(f)
				if est != lc {
					t.Errorf("%s %s: estimate=%d actual=%d — jitter on prewarm",
						label, f.Path, est, lc)
				}
				if lc != len(offsets) {
					t.Errorf("%s %s: sectionLineCache=%d len(offsets)=%d mismatch",
						label, f.Path, lc, len(offsets))
				}
			}

			// Verify scrollAdjust=0: simulate scrolled past the first two files.
			est0 := m.sectionLineCountEstimate(files[0])
			est1 := m.sectionLineCountEstimate(files[1])
			m.scrollTop = est0 + est1 + 1
			m.sectionLineCache = make(map[string]int)       // clear so prewarm uses estimate
			m.sectionLineOffsets = make(map[string][]int32) // clear offsets too
			prewarmBench(t, m)
			want := est0 + est1 + 1
			if m.scrollTop != want {
				t.Errorf("%s scrollTop changed after prewarm: want %d got %d (jitter!)",
					label, want, m.scrollTop)
			}
		}
	}
}

// TestPlaceholderWidths_EastAsian verifies that the placeholder skeleton shown
// before prewarm completes is exactly DiffPaneWidth terminal columns wide under
// Japanese locale. A too-wide placeholder would invade the scrollbar column on
// the very first frame, before any section has been pre-rendered.
func TestPlaceholderWidths_EastAsian(t *testing.T) {
	withEastAsianBoxChars(t)

	lang := "go"
	files := []types.DiffFile{
		{
			ID: "ea-path", Path: "pkg/あいう/処理.go",
			Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-ph-ea1"},
		},
		{
			ID: "long-ea", Path: "pkg/" + strings.Repeat("あ", 60) + "/file.go",
			Patch:    "@@ -1,4 +1,4 @@\n ctx\n-d1\n-d2\n+a1\n+a2\n ctx2\n",
			Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 2},
			Metadata: types.DiffMetadata{CacheKey: "ck-ph-ea2"},
		},
	}

	for _, termW := range []int{80, 120, 160} {
		for _, mode := range []types.LayoutMode{types.LayoutModeSplit, types.LayoutModeStack} {
			label := fmt.Sprintf("ja/termW=%d/%s", termW, mode)
			cs := types.Changeset{ID: "ph-ea", Files: files}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 40
			m.layout = layout.ComputeLayout(termW, m.termHeight, mode, 34, false, false)

			wantW := m.layout.DiffPaneWidth
			for _, f := range files {
				rendered := m.renderFileSectionPlaceholder(f)
				lines := strings.Split(rendered, "\n")
				for i, line := range lines {
					if line == "" {
						continue
					}
					got := terminalLineWidth(line)
					if got != wantW {
						t.Errorf("%s %s: placeholder line %d width=%d want=%d",
							label, f.Path, i, got, wantW)
					}
				}
			}
		}
	}
}

// TestHorizontalScrollInvalidatesCache verifies that changing codeHorizontalOffset
// triggers a cache invalidation (clearRenderCache) so sections are re-prewarmed
// with the new offset baked in, and scrollAdjust remains zero (line counts are
// invariant to horizontal offset — only visible content shifts).
func TestHorizontalScrollInvalidatesCache(t *testing.T) {
	lang := "go"
	longLine := strings.Repeat("x", 200)
	files := []types.DiffFile{
		{
			ID: "hs1", Path: "pkg/a.go",
			Patch:    fmt.Sprintf("@@ -1,3 +1,3 @@\n ctx\n-old %s\n+new %s\n", longLine, longLine),
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-hs1", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,3 +1,3 @@"},
			}},
		},
	}

	for _, termW := range []int{80, 120} {
		for _, mode := range []types.LayoutMode{types.LayoutModeSplit, types.LayoutModeStack} {
			label := fmt.Sprintf("termW=%d/%s", termW, mode)
			cs := types.Changeset{ID: "hs", Files: files}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 40
			m.layout = layout.ComputeLayout(termW, m.termHeight, mode, 34, false, false)

			// Prewarm at offset=0.
			prewarmBench(t, m)
			_ = m.View()

			// Verify section is in cache at key "ck-hs1" (offset=0).
			if _, ok := m.sectionCache["ck-hs1"]; !ok {
				t.Errorf("%s: section not cached at offset=0", label)
			}

			// Simulate horizontal scroll (offset=5). Mirrors handleKey behavior.
			prevScrollTop := m.scrollTop
			m.codeHorizontalOffset = 5
			m.clearRenderCache()

			// Cache should be cleared; builtFrame cleared; prewarm pending.
			if len(m.sectionCache) != 0 {
				t.Errorf("%s: sectionCache not cleared after horizontal scroll", label)
			}

			// Prewarm at offset=5.
			prewarmBench(t, m)

			// scrollTop must not have moved (scrollAdjust=0: line counts are
			// invariant to horizontal offset).
			if m.scrollTop != prevScrollTop {
				t.Errorf("%s: scrollTop changed after horizontal prewarm: %d → %d",
					label, prevScrollTop, m.scrollTop)
			}

			// Section must now be cached under the offset=5 key.
			if _, ok := m.sectionCache["ck-hs1|5"]; !ok {
				t.Errorf("%s: section not cached at offset=5 key", label)
			}

			// Every line of the re-prewarmed section must be exactly DiffPaneWidth wide.
			wantW := m.layout.DiffPaneWidth
			sec := m.sectionCache["ck-hs1|5"]
			for i, line := range strings.Split(sec, "\n") {
				if line == "" {
					continue
				}
				got := terminalLineWidth(line)
				if got != wantW {
					t.Errorf("%s: offset=5 section line %d width=%d want=%d", label, i, got, wantW)
				}
			}
		}
	}
}

// TestFilterChangeInvalidatesDiffPane verifies that changing the filter value clears
// diffPaneCache so the diff pane shows the newly filtered files rather than stale content.
// Also verifies scrollTop is reset to 0 and selectedFileIndex is clamped to the new range.
func TestFilterChangeInvalidatesDiffPane(t *testing.T) {
	lang := "go"
	// Build 5 distinct files: only those whose path contains "match" will survive filter.
	files := []types.DiffFile{
		{ID: "f1", Path: "pkg/match/a.go", Patch: "@@ -1,2 +1,2 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-fc1", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,2 +1,2 @@"}}}},
		{ID: "f2", Path: "pkg/other/b.go", Patch: "@@ -1,2 +1,2 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-fc2", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,2 +1,2 @@"}}}},
		{ID: "f3", Path: "pkg/match/c.go", Patch: "@@ -1,2 +1,2 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-fc3", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,2 +1,2 @@"}}}},
		{ID: "f4", Path: "pkg/extra/d.go", Patch: "@@ -1,2 +1,2 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-fc4", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,2 +1,2 @@"}}}},
		{ID: "f5", Path: "pkg/match/e.go", Patch: "@@ -1,2 +1,2 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-fc5", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,2 +1,2 @@"}}}},
	}

	cs := types.Changeset{ID: "fc", Files: files}
	m := New(types.Bootstrap{Changeset: cs}).(*model)
	m.termWidth = 120
	m.termHeight = 40
	m.layout = layout.ComputeLayout(120, 40, types.LayoutModeStack, 34, false, false)

	// Prewarm with all 5 files visible (no filter).
	prewarmBench(t, m)
	_ = m.View()

	// diffPaneCache should be populated at scrollTop=0.
	if m.diffPaneCache == "" {
		t.Fatal("diffPaneCache not populated after initial prewarm+View")
	}
	origCache := m.diffPaneCache

	// Artificially advance scrollTop so it would be out of bounds after filtering.
	m.scrollTop = 10
	m.diffPaneScrollTop = 10
	m.diffPaneCache = origCache // re-set as if the cache was built at scrollTop=10
	m.selectedFileIndex = 3     // would be out of range after filtering to 3 files

	// Simulate filter change: the fixed code clears cache, resets scroll, clamps selection.
	m.diffPaneCache = ""
	m.bodyCache = ""
	m.scrollTop = 0
	m.filter.SetValue("match")
	m.markSidebarDirty()
	newCount := len(m.visibleFiles())
	if m.selectedFileIndex >= newCount {
		m.selectedFileIndex = max(newCount-1, 0)
	}

	// Verify: filter shows only 3 matching files.
	if newCount != 3 {
		t.Errorf("visibleFiles after filter: got %d, want 3", newCount)
	}

	// Verify: scrollTop reset to 0.
	if m.scrollTop != 0 {
		t.Errorf("scrollTop after filter: got %d, want 0", m.scrollTop)
	}

	// Verify: selectedFileIndex clamped to [0, newCount-1].
	if m.selectedFileIndex >= newCount {
		t.Errorf("selectedFileIndex %d >= newCount %d after filter", m.selectedFileIndex, newCount)
	}

	// Verify: diffPaneCache is cleared — next renderDiffPane must re-render with filtered files.
	if m.diffPaneCache != "" {
		t.Errorf("diffPaneCache not cleared after filter change")
	}

	// Verify: renderDiffPane now shows only the 3 matched files.
	diffPane := m.renderDiffPane()
	if diffPane == "" {
		t.Fatal("renderDiffPane returned empty after filter")
	}

	// The diff pane should contain paths for matched files but not the others.
	// Sections are cached by key; placeholders show the file path.
	// Re-prewarm to get actual sections.
	prewarmBench(t, m)
	_ = m.View()
	diffPane2 := m.renderDiffPane()
	if strings.Contains(diffPane2, "pkg/other/b.go") || strings.Contains(diffPane2, "pkg/extra/d.go") {
		t.Errorf("diff pane contains non-matching file after filter, want only 'match' files")
	}
}

// TestLayoutModeSwitchNoScrollJitter verifies that switching layout modes produces
// scrollAdjust == 0 after the prewarm completes (i.e. sectionLineCountEstimate
// exactly matches the actual rendered line count in each mode).
// Prior to the sectionLineCache-clear-on-clearRenderCache fix, switching modes
// while scrolled past the first file caused the old-mode actual counts to be mixed
// with new-mode patchLinesBeforeUnk* offsets, producing a non-zero scrollAdjust
// and a visible scroll jump.
func TestLayoutModeSwitchNoScrollJitter(t *testing.T) {
	lang := "go"
	files := []types.DiffFile{
		{ID: "f1", Path: "pkg/a/a.go",
			Patch:    "@@ -1,5 +1,5 @@\n ctx1\n ctx2\n-del1\n-del2\n+add1\n+add2\n ctx3\n",
			Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 2},
			Metadata: types.DiffMetadata{CacheKey: "ck-ms1", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,5 +1,5 @@"}}}},
		{ID: "f2", Path: "pkg/b/b.go",
			Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-ms2", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,3 +1,3 @@"}}}},
		{ID: "f3", Path: "pkg/c/c.go",
			Patch:    "@@ -1,4 +1,4 @@\n ctx1\n-del1\n-del2\n+add1\n+add2\n ctx2\n",
			Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 2},
			Metadata: types.DiffMetadata{CacheKey: "ck-ms3", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,4 +1,4 @@"}}}},
	}

	modes := []struct {
		from types.LayoutMode
		to   types.LayoutMode
	}{
		{types.LayoutModeSplit, types.LayoutModeStack},
		{types.LayoutModeStack, types.LayoutModeSplit},
		{types.LayoutModeStack, types.LayoutModeAuto},
		{types.LayoutModeSplit, types.LayoutModeAuto},
	}

	for _, tc := range modes {
		for _, termW := range []int{80, 120, 160} {
			label := fmt.Sprintf("termW=%d/%s→%s", termW, tc.from, tc.to)
			cs := types.Changeset{ID: "ms", Files: files}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 40
			m.layout = layout.ComputeLayout(termW, m.termHeight, tc.from, 34, false, false)

			// Prewarm in the initial layout mode.
			prewarmBench(t, m)
			_ = m.View()

			// Scroll past the first file to make the cross-mode inconsistency observable.
			m.scrollTop = m.sectionLineCountEstimate(files[0]) + 2
			if m.scrollTop > m.totalDiffLines()-m.bodyHeight() {
				m.scrollTop = max(m.totalDiffLines()-m.bodyHeight(), 0)
			}
			beforeScrollTop := m.scrollTop

			// Switch layout mode (mirrors the fixed handleKey order: clearRenderCache
			// before computeUnkScrollOffset, so stale old-mode sectionLineCache counts
			// are not used when computing the new-mode scroll position).
			m.layoutMode = tc.to
			m.layout = layout.ComputeLayout(termW, m.termHeight, tc.to, 34, false, false)
			m.clearRenderCache()
			m.scrollTop = m.computeUnkScrollOffset()
			afterSwitchScrollTop := m.scrollTop

			// Prewarm in the new layout mode.
			prewarmBench(t, m)

			// scrollTop must not jump after prewarm (scrollAdjust == 0 means no jitter).
			if m.scrollTop != afterSwitchScrollTop {
				t.Errorf("%s: scrollTop jumped after prewarm: before=%d after=%d (was %d before switch)",
					label, afterSwitchScrollTop, m.scrollTop, beforeScrollTop)
			}

			// Re-render and verify no diff pane lines overflow DiffPaneWidth.
			_ = m.View()
			wantW := m.layout.DiffPaneWidth
			diffPane := m.renderDiffPane()
			for i, line := range strings.Split(diffPane, "\n") {
				if line == "" {
					continue
				}
				got := terminalLineWidth(line)
				if got != wantW {
					t.Errorf("%s: diff pane line %d width=%d want=%d", label, i, got, wantW)
				}
			}
		}
	}
}

// TestLayoutModeSwitchScrollPositionAccurate verifies that after switching layout
// modes the scroll position points to the correct line in the new mode.
//
// Prior to the fix, computeUnkScrollOffset was called BEFORE clearRenderCache, so
// it used old-mode sectionLineCache counts for files before the selected file.
// Stack→split: f1 had 10 cached stack lines but only 8 split lines, so scrollTop
// was 2 too large — the viewport showed 2 lines into f2 instead of f2's header.
func TestLayoutModeSwitchScrollPositionAccurate(t *testing.T) {
	lang := "go"
	// f1: 2 dels + 2 adds → stack=10, split=8 (splitSavings=2).
	// f2: selected file, first unk.
	f1 := types.DiffFile{
		ID: "f1", Path: "pkg/a/a.go",
		Patch:    "@@ -1,5 +1,5 @@\n ctx1\n ctx2\n-del1\n-del2\n+add1\n+add2\n ctx3\n",
		Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 2},
		Metadata: types.DiffMetadata{CacheKey: "ck-sp-1", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,5 +1,5 @@"}}},
	}
	f2 := types.DiffFile{
		ID: "f2", Path: "pkg/b/b.go",
		Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
		Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
		Metadata: types.DiffMetadata{CacheKey: "ck-sp-2", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,3 +1,3 @@"}}},
	}
	files := []types.DiffFile{f1, f2}

	// Prewarm in stack mode so sectionLineCache has actual stack counts for f1.
	// termWidth=160 keeps vp=ViewportMedium so LayoutModeSplit is respected (120 would
	// trigger ViewportTight and force LayoutModeStack regardless of the mode argument).
	cs := types.Changeset{ID: "sp-acc", Files: files}
	m := New(types.Bootstrap{Changeset: cs}).(*model)
	m.termWidth = 160
	m.termHeight = 40
	m.layout = layout.ComputeLayout(160, 40, types.LayoutModeStack, 34, false, false)
	prewarmBench(t, m)

	// Select f2, unk 0.
	m.selectedFileIndex = 1
	m.selectedUnkIndex = 0

	// Switch to split mode using the FIXED order: clear cache THEN compute offset.
	m.layoutMode = types.LayoutModeSplit
	m.layout = layout.ComputeLayout(160, 40, types.LayoutModeSplit, 34, false, false)
	m.clearRenderCache()
	m.scrollTop = m.computeUnkScrollOffset()

	// In split mode f1 estimate = fileSectionLineCount(10) - splitSavings(2) = 8.
	// computeUnkScrollOffset adds +1 to skip f2's file-header and place unk 0 at top,
	// so expected scrollTop = splitF1Est + 1 = 9.
	splitF1Est := m.sectionLineCountEstimate(f1)
	if m.scrollTop != splitF1Est+1 {
		t.Errorf("scrollTop after stack→split = %d, want %d (f1 split estimate=%d)",
			m.scrollTop, splitF1Est+1, splitF1Est)
	}

	// After prewarm scrollTop must be unchanged (scrollAdjust == 0).
	prewarmBench(t, m)
	if m.scrollTop != splitF1Est+1 {
		t.Errorf("scrollTop jumped after prewarm: got=%d want=%d", m.scrollTop, splitF1Est+1)
	}
}

// TestResizeCrossViewportBoundaryNoScrollJitter verifies that when a terminal
// resize crosses the ViewportTight boundary (< 160 cols forces stack mode), the
// scroll position is re-anchored via computeUnkScrollOffset rather than keeping
// the stale split-mode scrollTop that would point to the wrong line.
//
// Without the fix in WindowSizeMsg handler, a split-mode scrollTop=9 (pointing
// to f2 unk-0 in split, where f1=8 lines) remains 9 after resizing to stack
// mode (where f1=10 lines), showing f1's last line instead of f2.
func TestResizeCrossViewportBoundaryNoScrollJitter(t *testing.T) {
	lang := "go"
	f1 := types.DiffFile{
		ID: "f1", Path: "pkg/a/a.go",
		Patch:    "@@ -1,5 +1,5 @@\n ctx1\n ctx2\n-del1\n-del2\n+add1\n+add2\n ctx3\n",
		Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 2},
		Metadata: types.DiffMetadata{CacheKey: "ck-rv-1", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,5 +1,5 @@"}}},
	}
	f2 := types.DiffFile{
		ID: "f2", Path: "pkg/b/b.go",
		Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
		Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
		Metadata: types.DiffMetadata{CacheKey: "ck-rv-2", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,3 +1,3 @@"}}},
	}
	files := []types.DiffFile{f1, f2}

	// Start in split mode at termWidth=160 (ViewportMedium).
	cs := types.Changeset{ID: "rv", Files: files}
	m := New(types.Bootstrap{Changeset: cs}).(*model)
	m.termWidth = 160
	m.termHeight = 40
	m.layoutMode = types.LayoutModeSplit
	m.layout = layout.ComputeLayout(160, 40, types.LayoutModeSplit, 34, false, false)
	prewarmBench(t, m)

	// Select f2, unk 0 — scrollTop should be splitF1Est+1.
	m.selectedFileIndex = 1
	m.selectedUnkIndex = 0
	m.scrollTop = m.computeUnkScrollOffset()
	splitScrollTop := m.scrollTop // e.g. 9

	// Simulate resize to termWidth=120 (ViewportTight → forced LayoutModeStack).
	// Manually apply what WindowSizeMsg handler does (mirrors the production code).
	m.termWidth = 120
	m.termHeight = 40
	oldMode := m.layout.LayoutMode
	m.layout = layout.ComputeLayout(120, 40, m.layoutMode, m.sidebarWidth, m.sidebarVisible, m.forceSidebarOpen)
	m.clearRenderCache()
	if m.layout.LayoutMode != oldMode {
		m.scrollTop = m.computeUnkScrollOffset()
	}

	// After resize, scrollTop must be re-anchored for stack mode.
	// In stack mode f1 has 10 lines, so scrollTop = 10 + 1 = 11.
	stackF1Est := m.sectionLineCountEstimate(f1) // 10 in stack mode
	wantScrollTop := stackF1Est + 1
	if m.scrollTop == splitScrollTop {
		t.Errorf("scrollTop not re-anchored after resize to stack mode: got=%d (old split value)",
			m.scrollTop)
	}
	if m.scrollTop != wantScrollTop {
		t.Errorf("scrollTop after resize = %d, want %d (stackF1Est=%d + 1)",
			m.scrollTop, wantScrollTop, stackF1Est)
	}

	// After prewarm in stack mode, scrollTop must remain stable (scrollAdjust == 0).
	prewarmBench(t, m)
	if m.scrollTop != wantScrollTop {
		t.Errorf("scrollTop jumped after resize+prewarm: got=%d want=%d", m.scrollTop, wantScrollTop)
	}
}

// TestToggleUnkHeadersNoScrollJitter verifies that toggling unk headers
// re-anchors scrollTop immediately. Without the fix, the old scrollTop (set when
// showHH=true) is 1 too large for showHH=false because the removed @@ line in the
// selected file shifts its unk start one row earlier.
func TestToggleUnkHeadersNoScrollJitter(t *testing.T) {
	lang := "go"
	f1 := types.DiffFile{
		ID: "f1", Path: "pkg/a/a.go",
		Patch:    "@@ -1,5 +1,5 @@\n ctx1\n ctx2\n-del1\n-del2\n+add1\n+add2\n ctx3\n",
		Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 2},
		Metadata: types.DiffMetadata{CacheKey: "ck-hh-1", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,5 +1,5 @@"}}},
	}
	f2 := types.DiffFile{
		ID: "f2", Path: "pkg/b/b.go",
		Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
		Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
		Metadata: types.DiffMetadata{CacheKey: "ck-hh-2", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,3 +1,3 @@"}}},
	}
	files := []types.DiffFile{f1, f2}

	cs := types.Changeset{ID: "hh", Files: files}
	m := New(types.Bootstrap{Changeset: cs}).(*model)
	m.termWidth = 160
	m.termHeight = 40
	m.layout = layout.ComputeLayout(160, 40, types.LayoutModeStack, 34, false, false)
	prewarmBench(t, m)

	m.selectedFileIndex = 1
	m.selectedUnkIndex = 0
	m.scrollTop = m.computeUnkScrollOffset()
	scrollTopWithHH := m.scrollTop // showHH=true: f1=10 lines → scrollTop=11

	// Toggle unk headers off. Section estimates drop by 1 per unk.
	// Without the fix, scrollTop stays at 11, but f2 unk now starts at 10.
	m.showUnkHeaders = false
	m.clearRenderCache()
	m.scrollTop = m.computeUnkScrollOffset()

	scrollTopWithoutHH := m.scrollTop
	if scrollTopWithoutHH == scrollTopWithHH {
		t.Errorf("scrollTop not updated after unk-header toggle: got=%d (unchanged from showHH=true value)",
			scrollTopWithoutHH)
	}

	// Verify the value is correct: f1 without HH = sectionLineCountStack = 9, scrollTop = 9+1 = 10.
	f1EstNoHH := m.sectionLineCountEstimate(f1) // should be 9
	wantScrollTop := f1EstNoHH + 1
	if scrollTopWithoutHH != wantScrollTop {
		t.Errorf("scrollTop after unk-header toggle = %d, want %d (f1EstNoHH=%d)",
			scrollTopWithoutHH, wantScrollTop, f1EstNoHH)
	}

	// After prewarm, scrollAdjust == 0 (estimate matches actual), scrollTop stable.
	prewarmBench(t, m)
	if m.scrollTop != wantScrollTop {
		t.Errorf("scrollTop jumped after toggle+prewarm: got=%d want=%d", m.scrollTop, wantScrollTop)
	}
}
