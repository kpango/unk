package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/tui/textutil"
	"github.com/kpango/unk/internal/types"
	"github.com/mattn/go-runewidth"
)

// TestEAVisualRender_RuntimeLocale runs under the real process locale (not mocked).
// It renders full frames at every scroll position and verifies each line is exactly
// termWidth terminal columns wide — providing actual runtime evidence under LC_ALL=ja_JP.UTF-8.
func TestEAVisualRender_RuntimeLocale(t *testing.T) {
	lang := "go"
	longCode := strings.Repeat("x", 120)
	files := []types.DiffFile{
		{
			ID: "f1", Path: "pkg/あいう/処理.go",
			Patch:    fmt.Sprintf("@@ -1,4 +1,4 @@\n ctx1\n-old…%s\n+new…%s\n ctx2\n", longCode, longCode),
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "rv-f1",
				Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,4 +1,4 @@ func 処理中()"}},
			},
		},
		{
			ID: "f2", Path: "pkg/b/normal.go",
			Patch:    "@@ -1,5 +1,5 @@\n ctx\n-del1\n-del2\n+add1\n+add2\n ctx2\n",
			Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 2},
			Metadata: types.DiffMetadata{CacheKey: "rv-f2",
				Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,5 +1,5 @@"}},
			},
		},
	}

	ea := runewidth.DefaultCondition.EastAsianWidth
	t.Logf("runtime locale: EastAsianWidth=%v  │=%d  …=%d  ─=%d",
		ea, runewidth.RuneWidth('│'), runewidth.RuneWidth('…'), runewidth.RuneWidth('─'))

	for _, termW := range []int{80, 120, 160, 220} {
		for _, mode := range []types.LayoutMode{types.LayoutModeSplit, types.LayoutModeStack} {
			label := fmt.Sprintf("termW=%d/mode=%s", termW, mode)
			cs := types.Changeset{ID: "rv", Files: files, Title: "PR: 日本語テスト…の修正"}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 24
			m.layout = layout.ComputeLayout(termW, m.termHeight, mode, 34, false, false)
			prewarmBench(t, m)

			maxScroll := max(m.totalDiffLines()-m.bodyHeight(), 0)
			fail := 0
			for s := 0; s <= maxScroll; s++ {
				m.scrollTop = s
				m.diffPaneCache = ""
				m.bodyCache = ""
				m.menuBarCache = ""
				m.statusBarCache = ""
				m.menuBarDirty = true
				m.statusBarDirty = true
				frame := m.renderFull()
				// Print first frame so we can visually verify the layout.
				if s == 0 {
					t.Logf("=== %s scroll=0 frame (stripped ANSI) ===", label)
					for i, line := range strings.Split(frame, "\n") {
						bare := textutil.StripANSI(line)
						w := runewidth.StringWidth(bare)
						marker := ""
						if w != termW && bare != "" {
							marker = fmt.Sprintf(" ← WRONG (got %d want %d)", w, termW)
						}
						t.Logf("  [%2d] w=%3d |%s|%s", i, w, bare, marker)
					}
				}
				for i, line := range strings.Split(frame, "\n") {
					if line == "" {
						continue
					}
					got := terminalLineWidth(line)
					if got != termW {
						t.Errorf("%s scroll=%d line=%d: width=%d want=%d",
							label, s, i, got, termW)
						fail++
						if fail >= 3 {
							goto nextMode
						}
					}
				}
			}
			if fail == 0 {
				t.Logf("PASS %s: all %d scroll positions × all lines = %d cols", label, maxScroll+1, termW)
			}
		nextMode:
		}
	}
}
