package model

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/tui/textutil"
	"github.com/kpango/unk/internal/types"
	"github.com/mattn/go-runewidth"
)

// ansiRe strips all ANSI escape codes for terminal-width measurement.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// terminalLineWidth returns the terminal column width of a rendered line
// (stripping ANSI codes and using runewidth for locale-aware measurement).
func terminalLineWidth(s string) int { return runewidth.StringWidth(ansiRe.ReplaceAllString(s, "")) }

// withEastAsianBoxChars temporarily sets package layout vars to simulate a
// Japanese locale where box-drawing characters are 2 terminal columns wide.
// Returns a cleanup function to restore the originals.
func withEastAsianBoxChars(t *testing.T) {
	t.Helper()
	origBoxCharW := layout.BoxCharW
	origEllipsisW := layout.EllipsisW
	origDivW := layout.DividerWidth
	origBodyP := layout.BodyPadding
	origForceSW := layout.ForceSidebarMinWidth
	origEA := runewidth.DefaultCondition.EastAsianWidth

	// Simulate ja_JP locale: box-drawing and ellipsis chars are 2 terminal cols each.
	runewidth.DefaultCondition.EastAsianWidth = true
	layout.BoxCharW = 2
	layout.EllipsisW = 2
	layout.DividerWidth = 2
	layout.BodyPadding = 2
	layout.ForceSidebarMinWidth = layout.SidebarMinWidth + 2 + layout.DiffMinWidth

	t.Cleanup(func() {
		layout.BoxCharW = origBoxCharW
		layout.EllipsisW = origEllipsisW
		layout.DividerWidth = origDivW
		layout.BodyPadding = origBodyP
		layout.ForceSidebarMinWidth = origForceSW
		runewidth.DefaultCondition.EastAsianWidth = origEA
	})
}

// TestColFill verifies that colFill generates exactly the requested number of
// terminal columns, both in narrow (ASCII/en_US) and wide (ja_JP) locale modes.
func TestColFill(t *testing.T) {
	cases := []struct {
		ch      rune
		cols    int
		charW   int // simulated runewidth of ch
		wantN   int // expected number of ch runes in output
		wantRem int // expected trailing spaces
	}{
		{'─', 10, 1, 10, 0}, // narrow: 10 chars × 1 col = 10 cols
		{'─', 10, 2, 5, 0},  // wide:   5 chars × 2 col = 10 cols
		{'─', 11, 2, 5, 1},  // wide, odd: 5 chars × 2 + 1 space = 11 cols
		{'─', 0, 1, 0, 0},   // zero cols → empty
		{'─', 1, 2, 0, 1},   // 1 col, wide char → 0 chars + 1 space
	}
	for _, tc := range cases {
		origEA := runewidth.DefaultCondition.EastAsianWidth
		if tc.charW == 2 {
			runewidth.DefaultCondition.EastAsianWidth = true
		} else {
			runewidth.DefaultCondition.EastAsianWidth = false
		}
		got := textutil.ColFill(tc.ch, tc.cols)
		// Measure terminal width under the SAME EastAsianWidth setting used to generate.
		gotW := runewidth.StringWidth(got)
		runewidth.DefaultCondition.EastAsianWidth = origEA // restore after measurement

		if tc.cols == 0 {
			if got != "" {
				t.Errorf("textutil.ColFill(%q, 0) = %q, want empty", tc.ch, got)
			}
			continue
		}
		if gotW != tc.cols {
			t.Errorf("textutil.ColFill(%q, %d) terminal width = %d, want %d", tc.ch, tc.cols, gotW, tc.cols)
		}
	}
}

// TestScrollbarColumnStable_EastAsian repeats the scrollbar stability check under
// simulated Japanese locale (box chars = 2 terminal columns each).
//
// Every diffPane line must be exactly DiffPaneWidth TERMINAL columns wide.
// Under East Asian locale, lipgloss.Width() undercounts box chars (treats them
// as 1 col while the terminal renders them as 2). We therefore measure with
// runewidth.StringWidth(stripANSI(line)) which is locale-aware.
func TestScrollbarColumnStable_EastAsian(t *testing.T) {
	withEastAsianBoxChars(t)

	lang := "go"
	longCode := strings.Repeat("x", 200)
	longPath := "very/deeply/nested/" + strings.Repeat("z", 120) + ".go"

	files := []types.DiffFile{
		{
			ID: "long-lines", Path: "pkg/longlines/file.go",
			Patch:    fmt.Sprintf("@@ -1,4 +1,4 @@ func processLong()\n ctx\n-old: %s\n+new: %s\n ctx2\n\n", longCode, longCode),
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-ea-ll", Unks: []types.DiffUnk{
				{Index: 0, Header: fmt.Sprintf("@@ -1,4 +1,4 @@ func processLong_%s(x int)", strings.Repeat("a", 120))},
			}},
		},
		{
			ID: "blank-lines", Path: "pkg/blank/file.go",
			Patch:    "@@ -1,4 +1,4 @@\n ctx\n-del\n+add\n ctx2\n\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-ea-bl", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,4 +1,4 @@"},
			}},
		},
		{
			ID: "stack-sep", Path: "pkg/sep/file.go",
			Patch:    "@@ -1,4 +1,4 @@\n ctx\n-del1\n-del2\n+add1\n+add2\n ctx2\n",
			Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 2},
			Metadata: types.DiffMetadata{CacheKey: "ck-ea-sep", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,4 +1,4 @@"},
			}},
		},
		{
			ID: "long-path", Path: longPath,
			Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-ea-lp", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,3 +1,3 @@"},
			}},
		},
		{
			// EA ambiguous chars ("…") in code content: runewidth=2 under ja locale,
			// lipgloss.Width=1. stack view writeWidthTo must use visibleWidth, not
			// lipgloss width, to pad correctly.
			ID: "ellipsis-content", Path: "pkg/ellipsis/file.go",
			Patch:    "@@ -1,3 +1,3 @@\n // 処理中…\n-old_val…\n+new_val…\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-ea-el", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,3 +1,3 @@ func ellipsis…()"},
			}},
		},
	}

	for _, termW := range []int{80, 120, 160, 220} {
		for _, mode := range []types.LayoutMode{types.LayoutModeSplit, types.LayoutModeStack} {
			label := fmt.Sprintf("ja/termW=%d/%s", termW, mode)
			cs := types.Changeset{ID: "ea-scroll", Files: files}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 40
			m.layout = layout.ComputeLayout(m.termWidth, m.termHeight, mode, 34, false, false)

			prewarmBench(t, m)
			_ = m.View()

			wantW := m.layout.DiffPaneWidth
			maxScroll := m.totalDiffLines() - m.bodyHeight()
			maxScroll = max(maxScroll, 0)

			checkScrollPos := func(scrollTop int) {
				m.scrollTop = scrollTop
				m.diffPaneCache = ""
				diffPane := m.renderDiffPane()
				lines := strings.Split(diffPane, "\n")
				failures := 0
				for i, line := range lines {
					if line == "" {
						continue
					}
					got := terminalLineWidth(line)
					if got != wantW {
						t.Errorf("%s scroll=%d: line %d terminal-width=%d want=%d",
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

// TestMenuBarStatusBarWidth_EastAsian verifies that the menu bar and status bar
// are exactly termWidth terminal columns wide under Japanese locale, even when
// the PR title or filter text contains EA Ambiguous characters (RC 25).
//
// Root cause: gap = max(termWidth - lipgloss.Width(left) - rightW, 0) in
// renderMenuBarInner undercounted EA Ambiguous chars in the title, making gap too
// large and the bar overflow by N. Similarly renderStatusBarInner used
// Width(n).Render(content) for padding which over-pads under EA locale.
func TestMenuBarStatusBarWidth_EastAsian(t *testing.T) {
	withEastAsianBoxChars(t)

	lang := "go"
	files := []types.DiffFile{{
		ID: "f1", Path: "pkg/a/file.go",
		Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
		Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
		Metadata: types.DiffMetadata{CacheKey: "ck-mb-ea", Unks: []types.DiffUnk{
			{Index: 0, Header: "@@ -1,3 +1,3 @@"},
		}},
	}}

	// PR title and notice text with EA Ambiguous characters.
	eaTitles := []string{
		"PR: fix" + "\xe2\x80\xa6" + "bug in処理中", // "…" + CJK
		strings.Repeat("あ", 10),                  // all CJK (2-col per char)
		"short",                                  // ASCII baseline
	}
	eaFilters := []string{
		"日本語filter",
		"normalfilter",
	}

	for _, termW := range []int{80, 120, 160} {
		for _, eaTitle := range eaTitles {
			cs := types.Changeset{ID: "mb-ea", Files: files, Title: eaTitle}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 40
			m.layout = layout.ComputeLayout(termW, m.termHeight, types.LayoutModeStack, 34, false, false)

			bar := m.renderMenuBarInner()
			got := terminalLineWidth(bar)
			if got != termW {
				t.Errorf("menuBar termW=%d title=%q: width=%d want=%d", termW, eaTitle, got, termW)
			}
		}

		for _, filter := range eaFilters {
			cs := types.Changeset{ID: "sb-ea", Files: files}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 40
			m.layout = layout.ComputeLayout(termW, m.termHeight, types.LayoutModeStack, 34, false, false)
			m.filter.SetValue(filter)

			bar := m.renderStatusBarInner()
			got := terminalLineWidth(bar)
			if got != termW {
				t.Errorf("statusBar termW=%d filter=%q: width=%d want=%d", termW, filter, got, termW)
			}
		}
	}
}

// TestFullFrameRowWidths_EastAsian_WithSidebar verifies full-frame row widths under
// Japanese locale when the sidebar column is actually visible (sidebarVisible=true,
// forceSidebarOpen=true). The sidebar adds a │ divider that is 2 terminal columns
// wide under EA locale; joinColumnsAll must still produce exactly termWidth columns.
func TestFullFrameRowWidths_EastAsian_WithSidebar(t *testing.T) {
	withEastAsianBoxChars(t)

	lang := "go"
	longCode := strings.Repeat("x", 200)
	files := []types.DiffFile{
		{
			ID: "f1", Path: "pkg/あ/file.go",
			Patch:    fmt.Sprintf("@@ -1,4 +1,4 @@\n ctx\n-old…%s\n+new…%s\n ctx2\n", longCode, longCode),
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-ws-ea-1", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,4 +1,4 @@ func 処理…()"},
			}},
		},
		{
			ID: "f2", Path: "pkg/b/normal.go",
			Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-ws-ea-2", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,3 +1,3 @@"},
			}},
		},
		{
			ID: "f3", Path: "pkg/c/处理中.go",
			Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 3},
			Metadata: types.DiffMetadata{CacheKey: "ck-ws-ea-3", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,3 +1,3 @@"},
			}},
		},
	}
	eaTitle := "PR: 処理中…の修正 sidebar-visible"

	// layout.ForceSidebarMinWidth under EA = layout.SidebarMinWidth(22) + layout.BoxCharW(2) + layout.DiffMinWidth(48) = 72.
	// Use termW values that span: sidebar forced open (80), medium (120), full-vp (220).
	for _, termW := range []int{80, 120, 160, 220} {
		for _, mode := range []types.LayoutMode{types.LayoutModeSplit, types.LayoutModeStack} {
			label := fmt.Sprintf("ja/sidebar/termW=%d/%s", termW, mode)
			cs := types.Changeset{ID: "ws-ea", Files: files, Title: eaTitle}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 40
			// sidebarVisible=true, forceSidebarOpen=true → sidebar always rendered.
			m.layout = layout.ComputeLayout(termW, m.termHeight, mode, 34, true, true)

			if !m.layout.RenderSidebar {
				t.Logf("%s: sidebar not rendered (termW too small), skipping", label)
				continue
			}

			prewarmBench(t, m)
			_ = m.View()

			maxScroll := max(m.totalDiffLines()-m.bodyHeight(), 0)

			checkFrame := func(scrollTop int) {
				m.scrollTop = scrollTop
				m.diffPaneCache = ""
				m.bodyCache = ""
				m.menuBarCache = ""
				m.statusBarCache = ""
				m.menuBarDirty = true
				m.statusBarDirty = true
				m.sidebarRowsDirty = true
				frame := m.renderFull()
				lines := strings.Split(frame, "\n")
				failures := 0
				for i, line := range lines {
					if line == "" {
						continue
					}
					got := terminalLineWidth(line)
					if got != termW {
						t.Errorf("%s scroll=%d: frame line %d width=%d want=%d",
							label, scrollTop, i, got, termW)
						failures++
						if failures >= 3 {
							return
						}
					}
				}
			}

			for s := 0; s <= maxScroll; s++ {
				checkFrame(s)
			}
		}
	}
}

// TestFullFrameRowWidths_EastAsian verifies that every line in the fully assembled
// frame (menu bar + body rows + status bar) is exactly termWidth terminal columns
// wide under Japanese locale, at multiple scroll positions.
//
// This catches any overflow that escapes per-component tests: sidebar, divider,
// diffPane, and scrollbar are assembled by joinColumnsAll — if any component is
// off by even 1 column, the terminal wraps that row and displaces the scrollbar.
func TestFullFrameRowWidths_EastAsian(t *testing.T) {
	withEastAsianBoxChars(t)

	lang := "go"
	longCode := strings.Repeat("x", 200)
	files := []types.DiffFile{
		{
			ID: "f1", Path: "pkg/あ/file.go",
			Patch:    fmt.Sprintf("@@ -1,4 +1,4 @@\n ctx\n-old…%s\n+new…%s\n ctx2\n", longCode, longCode),
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-ff-ea-1", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,4 +1,4 @@ func 処理…()"},
			}},
		},
		{
			ID: "f2", Path: "pkg/b/normal.go",
			Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-ff-ea-2", Unks: []types.DiffUnk{
				{Index: 0, Header: "@@ -1,3 +1,3 @@"},
			}},
		},
	}
	eaTitle := "PR: 処理中…の修正 fix─overflow"

	for _, termW := range []int{80, 120, 220} {
		for _, mode := range []types.LayoutMode{types.LayoutModeSplit, types.LayoutModeStack} {
			label := fmt.Sprintf("ja/termW=%d/%s", termW, mode)
			cs := types.Changeset{ID: "ff-ea", Files: files, Title: eaTitle}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 40
			m.layout = layout.ComputeLayout(termW, m.termHeight, mode, 34, false, false)

			prewarmBench(t, m)
			_ = m.View()

			maxScroll := max(m.totalDiffLines()-m.bodyHeight(), 0)

			checkFrame := func(scrollTop int) {
				m.scrollTop = scrollTop
				m.diffPaneCache = ""
				m.bodyCache = ""
				m.menuBarCache = ""
				m.statusBarCache = ""
				m.menuBarDirty = true
				m.statusBarDirty = true
				frame := m.renderFull()
				lines := strings.Split(frame, "\n")
				failures := 0
				for i, line := range lines {
					if line == "" {
						continue
					}
					got := terminalLineWidth(line)
					if got != termW {
						t.Errorf("%s scroll=%d: frame line %d width=%d want=%d",
							label, scrollTop, i, got, termW)
						failures++
						if failures >= 3 {
							return
						}
					}
				}
			}

			for s := 0; s <= maxScroll; s++ {
				checkFrame(s)
			}
		}
	}
}
