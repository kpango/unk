package model

// TestSessionStress_NoScrollJitter simulates complete user sessions (layout
// switches + navigation + prewarm) across many terminal widths and file
// configurations, verifying scrollAdjust==0 (no jitter) in every case and
// every full-frame line is exactly termWidth terminal columns wide.
//
// This test runs under both en_US and ja_JP locales without mocking, using
// the real runtime locale settings (layout.BoxCharW, layout.EllipsisW, etc.).

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/types"
)

func TestSessionStress_NoScrollJitter(t *testing.T) {
	lang := "go"
	// Representative patches: single-unk, multi-unk, del-only, add-only,
	// mixed context, many empty lines.
	patches := []struct {
		name  string
		patch string
	}{
		{"single-bal", "@@ -1,4 +1,4 @@\n ctx1\n-del\n+add\n ctx2\n"},
		{"single-del-only", "@@ -1,3 +1,2 @@\n ctx\n-del1\n-del2\n"},
		{"single-add-only", "@@ -1,2 +1,3 @@\n ctx\n+add1\n+add2\n"},
		{"multi-unk", "@@ -1,4 +1,4 @@\n ctx1\n-del1\n+add1\n ctx2\n@@ -10,4 +10,4 @@\n ctx3\n-del2\n+add2\n ctx4\n"},
		{"3-unk", "@@ -1,3 +1,3 @@\n-d1\n+a1\n c1\n@@ -5,3 +5,3 @@\n-d2\n+a2\n c2\n@@ -10,3 +10,3 @@\n-d3\n+a3\n c3\n"},
		{"asymmetric", "@@ -1,5 +1,3 @@\n ctx\n-del1\n-del2\n-del3\n+add1\n ctx2\n"},
		{"cjk-path-file", "@@ -1,3 +1,3 @@\n ctx1\n-あいう\n+えおか\n"},
	}

	files := make([]types.DiffFile, len(patches))
	for i, p := range patches {
		files[i] = types.DiffFile{
			ID: fmt.Sprintf("f%d", i), Path: fmt.Sprintf("pkg/%s.go", p.name),
			Patch:    p.patch,
			Language: &lang,
			Stats:    types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{
				CacheKey: fmt.Sprintf("ck-%d", i),
				Unks:     []types.DiffUnk{{Index: 0, Header: "@@ -1,4 +1,4 @@"}},
			},
		}
	}

	termWidths := []int{80, 100, 120, 140, 160, 180, 200, 220}
	modes := []types.LayoutMode{types.LayoutModeSplit, types.LayoutModeStack}
	hhModes := []bool{true, false}

	for _, termW := range termWidths {
		for _, mode := range modes {
			for _, showHH := range hhModes {
				label := fmt.Sprintf("w=%d/%s/hh=%v", termW, mode, showHH)

				cs := types.Changeset{ID: "stress", Files: files}
				m := New(types.Bootstrap{Changeset: cs}).(*model)
				m.termWidth = termW
				m.termHeight = 30
				m.showUnkHeaders = showHH
				m.layout = layout.ComputeLayout(termW, m.termHeight, mode, 34, false, false)

				// Initial prewarm.
				prewarmBench(t, m)

				// Navigate to each file and verify no jitter after simulated
				// clearRenderCache + computeUnkScrollOffset + prewarm cycle.
				for fi := 0; fi < len(files); fi++ {
					m.selectedFileIndex = fi
					m.selectedUnkIndex = 0

					// Simulate a layout-switch (most likely to cause jitter).
					newMode := types.LayoutModeStack
					if mode == types.LayoutModeStack {
						newMode = types.LayoutModeSplit
					}
					m.layoutMode = newMode
					m.layout = layout.ComputeLayout(termW, m.termHeight, newMode, 34, false, false)
					m.clearRenderCache()
					m.scrollTop = m.computeUnkScrollOffset()
					scrollBeforePrewarm := m.scrollTop

					prewarmBench(t, m)

					if m.scrollTop != scrollBeforePrewarm {
						t.Errorf("%s fi=%d: jitter after layout-switch+prewarm: before=%d after=%d",
							label, fi, scrollBeforePrewarm, m.scrollTop)
					}

					// Restore original mode for next iteration.
					m.layoutMode = mode
					m.layout = layout.ComputeLayout(termW, m.termHeight, mode, 34, false, false)
					m.clearRenderCache()
					m.scrollTop = m.computeUnkScrollOffset()
					prewarmBench(t, m)
				}

				// Verify every full-frame line is exactly termWidth cols.
				maxScroll := max(m.totalDiffLines()-m.bodyHeight(), 0)
				fail := 0
				for s := 0; s <= min(maxScroll, 10); s++ {
					m.scrollTop = s
					m.diffPaneCache = ""
					m.bodyCache = ""
					frame := m.renderFull()
					for li, line := range strings.Split(frame, "\n") {
						if line == "" {
							continue
						}
						got := terminalLineWidth(line)
						if got != termW {
							t.Errorf("%s scroll=%d line=%d: width=%d want=%d",
								label, s, li, got, termW)
							fail++
							if fail >= 3 {
								goto nextCase
							}
						}
					}
				}
			nextCase:
			}
		}
	}
}

// TestSessionStress_WindowResizeNoJitter verifies that resizing the terminal
// across the ViewportTight boundary (160 cols) always re-anchors scrollTop
// and produces zero scrollAdjust on the following prewarm.
func TestSessionStress_WindowResizeNoJitter(t *testing.T) {
	lang := "go"
	files := []types.DiffFile{
		{ID: "f1", Path: "a.go",
			Patch:    "@@ -1,5 +1,5 @@\n ctx1\n ctx2\n-del1\n-del2\n+add1\n+add2\n ctx3\n",
			Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 2},
			Metadata: types.DiffMetadata{CacheKey: "ck-r1", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,5 +1,5 @@"}}}},
		{ID: "f2", Path: "b.go",
			Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "ck-r2", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,3 +1,3 @@"}}}},
	}

	resizePairs := [][2]int{
		{160, 120}, // split→stack (cross tight boundary downward)
		{120, 160}, // stack→split (cross tight boundary upward)
		{160, 80},
		{220, 120},
		{220, 80},
	}

	for _, rp := range resizePairs {
		fromW, toW := rp[0], rp[1]
		label := fmt.Sprintf("resize %d→%d", fromW, toW)

		cs := types.Changeset{ID: "resize", Files: files}
		m := New(types.Bootstrap{Changeset: cs}).(*model)
		m.termWidth = fromW
		m.termHeight = 40
		m.layoutMode = types.LayoutModeSplit
		m.layout = layout.ComputeLayout(fromW, 40, types.LayoutModeSplit, 34, false, false)
		prewarmBench(t, m)

		// Navigate to f2.
		m.selectedFileIndex = 1
		m.selectedUnkIndex = 0
		m.scrollTop = m.computeUnkScrollOffset()

		// Simulate WindowSizeMsg (mirrors the production handler).
		oldMode := m.layout.LayoutMode
		m.termWidth = toW
		m.layout = layout.ComputeLayout(toW, 40, m.layoutMode, m.sidebarWidth, m.sidebarVisible, m.forceSidebarOpen)
		m.clearRenderCache()
		if m.layout.LayoutMode != oldMode {
			m.scrollTop = m.computeUnkScrollOffset()
		}
		scrollAfterResize := m.scrollTop

		// Prewarm and verify no jitter.
		prewarmBench(t, m)
		if m.scrollTop != scrollAfterResize {
			t.Errorf("%s: scrollTop jumped after resize+prewarm: before=%d after=%d",
				label, scrollAfterResize, m.scrollTop)
		}

		// Process resize as a tea.WindowSizeMsg to exercise the real handler.
		m2, _ := m.Update(tea.WindowSizeMsg{Width: toW, Height: 40})
		m3 := m2.(*model)
		prewarmBench(t, m3)
		// scrollTop should still be anchored at f2.
		if m3.selectedFileIndex == 1 {
			// f2 is still selected; scrollTop should point into f2 (not before it).
			f1Size := m3.sectionLineCountEstimate(files[0])
			if m3.scrollTop < f1Size {
				// Only warn if sectionLineCache is populated (estimate may differ).
				if _, ok := m3.sectionLineCache[m3.sectionCacheKey(files[0])]; ok {
					if m3.scrollTop < m3.sectionLineCache[m3.sectionCacheKey(files[0])] {
						t.Errorf("%s: after full resize+prewarm scrollTop=%d < f1Size=%d (f2 not visible)",
							label, m3.scrollTop, f1Size)
					}
				}
			}
		}
	}
}

// TestScrollDuringPrewarmNoJitter verifies that if the user scrolls between a
// layout switch (clearRenderCache) and the prewarm completing, the subsequent
// prewarmMsg scrollAdjust is still 0 — the adjustment must be computed relative
// to wherever scrollTop actually is when the prewarmMsg arrives.
func TestScrollDuringPrewarmNoJitter(t *testing.T) {
	lang := "go"
	files := []types.DiffFile{
		{ID: "f1", Path: "a.go",
			Patch:    "@@ -1,5 +1,5 @@\n ctx1\n ctx2\n-del1\n-del2\n+add1\n+add2\n ctx3\n",
			Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 2},
			Metadata: types.DiffMetadata{CacheKey: "sd-f1", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,5 +1,5 @@"}}}},
		{ID: "f2", Path: "b.go",
			Patch:    "@@ -1,3 +1,3 @@\n ctx\n-del\n+add\n",
			Language: &lang, Stats: types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "sd-f2", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,3 +1,3 @@"}}}},
		{ID: "f3", Path: "c.go",
			Patch:    "@@ -1,4 +1,4 @@\n ctx\n-del1\n-del2\n+add1\n+add2\n",
			Language: &lang, Stats: types.DiffStats{Additions: 2, Deletions: 2},
			Metadata: types.DiffMetadata{CacheKey: "sd-f3", Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,4 +1,4 @@"}}}},
	}

	for _, termW := range []int{80, 120, 160} {
		for _, mode := range []types.LayoutMode{types.LayoutModeSplit, types.LayoutModeStack} {
			label := fmt.Sprintf("w=%d/%s", termW, mode)
			cs := types.Changeset{ID: "sd", Files: files}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 30
			m.layout = layout.ComputeLayout(termW, m.termHeight, mode, 34, false, false)

			// Simulate layout switch: clears render cache, sets scrollTop to estimate.
			m.clearRenderCache()
			m.scrollTop = m.computeUnkScrollOffset()
			anchorBeforeScroll := m.scrollTop

			// Simulate user scrolling DOWN by 3 lines while prewarm is pending.
			m.scrollTop = min(m.scrollTop+3, max(m.totalDiffLines()-m.bodyHeight(), 0))
			scrollAfterScroll := m.scrollTop

			// Now run prewarm (this is what prewarmMsg delivers).
			prewarmBench(t, m)

			// scrollTop must not have changed (scrollAdjust must be 0).
			if m.scrollTop != scrollAfterScroll {
				t.Errorf("%s: jitter after scroll+prewarm: anchor=%d scrolled=%d after=%d",
					label, anchorBeforeScroll, scrollAfterScroll, m.scrollTop)
			}

			// Also verify full-frame lines at the scrolled position are exactly termW wide.
			m.diffPaneCache = ""
			m.bodyCache = ""
			frame := m.renderFull()
			fail := 0
			for li, line := range strings.Split(frame, "\n") {
				if line == "" {
					continue
				}
				got := terminalLineWidth(line)
				if got != termW {
					t.Errorf("%s: line %d width=%d want=%d", label, li, got, termW)
					fail++
					if fail >= 3 {
						break
					}
				}
			}
		}
	}
}

// TestScrollbarGeomStableAcrossPrewarm verifies that totalDiffLines() is the same
// before and after prewarm, so the scrollbar thumb position never jumps when
// pre-rendered sections replace placeholders.
func TestScrollbarGeomStableAcrossPrewarm(t *testing.T) {
	lang := "go"
	patches := []struct{ name, patch string }{
		{"bal", "@@ -1,4 +1,4 @@\n ctx\n-del\n+add\n ctx2\n"},
		{"del-only", "@@ -1,3 +1,2 @@\n ctx\n-del1\n-del2\n"},
		{"add-only", "@@ -1,2 +1,3 @@\n ctx\n+add1\n+add2\n"},
		{"2unk", "@@ -1,3 +1,3 @@\n-d1\n+a1\n c\n@@ -5,3 +5,3 @@\n-d2\n+a2\n c2\n"},
	}

	files := make([]types.DiffFile, len(patches))
	for i, p := range patches {
		files[i] = types.DiffFile{
			ID: fmt.Sprintf("f%d", i), Path: p.name + ".go",
			Patch: p.patch, Language: &lang,
			Stats:    types.DiffStats{Additions: 1, Deletions: 1},
			Metadata: types.DiffMetadata{CacheKey: "sb-" + p.name},
		}
	}

	for _, termW := range []int{80, 120, 160, 220} {
		for _, mode := range []types.LayoutMode{types.LayoutModeSplit, types.LayoutModeStack} {
			label := fmt.Sprintf("w=%d/%s", termW, mode)
			cs := types.Changeset{ID: "sb", Files: files}
			m := New(types.Bootstrap{Changeset: cs}).(*model)
			m.termWidth = termW
			m.termHeight = 30
			m.layout = layout.ComputeLayout(termW, m.termHeight, mode, 34, false, false)

			// Measure totalDiffLines BEFORE prewarm (all estimates).
			totalBefore := m.totalDiffLines()

			// Run prewarm synchronously.
			prewarmBench(t, m)

			// Measure totalDiffLines AFTER prewarm (actual cached counts).
			totalAfter := m.totalDiffLines()

			if totalBefore != totalAfter {
				t.Errorf("%s: totalDiffLines before=%d after=%d (scrollbar thumb would jump)",
					label, totalBefore, totalAfter)
			}
		}
	}
}
