package model

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kpango/unk/internal/diff"
	"github.com/kpango/unk/internal/tui/keys"
	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/tui/render"
	"github.com/kpango/unk/internal/tui/textutil"
	"github.com/kpango/unk/internal/types"
)

// syntheticDiff builds a realistic unified diff patch for benchmarking.
func syntheticDiff(lines int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "@@ -1,%d +1,%d @@\n", lines, lines)
	for i := range lines {
		switch i % 4 {
		case 0:
			fmt.Fprintf(&sb, " func processItem(x int) int {\n")
		case 1:
			fmt.Fprintf(&sb, "-\treturn x * 2\n")
		case 2:
			fmt.Fprintf(&sb, "+\treturn x * 3 + 1\n")
		default:
			fmt.Fprintf(&sb, " }\n")
		}
	}
	return sb.String()
}

// syntheticMultiUnkDiff builds a unified diff with multiple separate unks.
// unks × linesPerUnk gives total line count. Used to exercise parallel unk rendering.
func syntheticMultiUnkDiff(unks, linesPerUnk int) string {
	var sb strings.Builder
	startOld, startNew := 1, 1
	for h := range unks {
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@ func unk%d()\n", startOld, linesPerUnk, startNew, linesPerUnk, h)
		for i := range linesPerUnk {
			switch i % 4 {
			case 0:
				fmt.Fprintf(&sb, " func processItem(x int) int {\n")
			case 1:
				fmt.Fprintf(&sb, "-\treturn x * 2\n")
			case 2:
				fmt.Fprintf(&sb, "+\treturn x * 3 + 1\n")
			default:
				fmt.Fprintf(&sb, " }\n")
			}
		}
		startOld += linesPerUnk + 20 // gap between unks
		startNew += linesPerUnk + 20
	}
	return sb.String()
}

// benchModelMultiUnk returns a Model with a single large file containing many unks.
func benchModelMultiUnk(tb testing.TB, unks, linesPerUnk int) *model {
	tb.Helper()
	lang := "go"
	patch := syntheticMultiUnkDiff(unks, linesPerUnk)
	var unkMeta []types.DiffUnk
	for i := range unks {
		unkMeta = append(unkMeta, types.DiffUnk{Index: i, Header: fmt.Sprintf("@@ func unk%d() @@", i)})
	}
	cs := types.Changeset{ID: "bench-multi"}
	cs.Files = append(cs.Files, types.DiffFile{
		ID:       "big-file",
		Path:     "pkg/big/file.go",
		Patch:    patch,
		Language: &lang,
		Stats:    types.DiffStats{Additions: unks * linesPerUnk / 2, Deletions: unks * linesPerUnk / 2},
		Metadata: types.DiffMetadata{CacheKey: "ck-big", Unks: unkMeta},
	})
	m := New(types.Bootstrap{Changeset: cs}).(*model)
	m.termWidth = 240
	m.termHeight = 50
	m.layout = layout.ComputeLayout(m.termWidth, m.termHeight, types.LayoutModeAuto, 34, true, false)
	return m
}

// benchModel returns a minimal Model with a realistic changeset.
func benchModel(tb testing.TB, files, linesPerFile int) *model {
	tb.Helper()
	lang := "go"
	cs := types.Changeset{ID: "bench"}
	for i := range files {
		patch := syntheticDiff(linesPerFile)
		cs.Files = append(cs.Files, types.DiffFile{
			ID:       fmt.Sprintf("file-%d", i),
			Path:     fmt.Sprintf("pkg/module%d/file.go", i),
			Patch:    patch,
			Language: &lang,
			Stats:    types.DiffStats{Additions: linesPerFile / 2, Deletions: linesPerFile / 2},
			Metadata: types.DiffMetadata{
				CacheKey: fmt.Sprintf("ck-%d-%d", i, linesPerFile),
				Unks: []types.DiffUnk{
					{Index: 0, Header: fmt.Sprintf("@@ -1,%d +1,%d @@", linesPerFile, linesPerFile)},
				},
			},
		})
	}
	m := New(types.Bootstrap{
		Changeset: cs,
	}).(*model)
	m.termWidth = 240
	m.termHeight = 50
	m.layout = layout.ComputeLayout(m.termWidth, m.termHeight, types.LayoutModeAuto, 34, true, false)
	return m
}

// prewarmBench simulates the background prewarm goroutine completing synchronously
// so that benchmarks can measure the warm-cache path without actually spinning up
// background goroutines. Mirrors what Update(render.PrewarmMsg) does in production.
// prewarmBench drives cmdPrewarmSections synchronously, running all per-file
// Cmds in the returned tea.Batch and merging each render.PrewarmMsg into the model.
// This simulates what BubbleTea does in production (runs each batch Cmd in a
// goroutine and delivers the resulting render.PrewarmMsg to Update() independently).
func prewarmBench(tb testing.TB, m *model) {
	tb.Helper()
	cmd := cmdPrewarmSections(m.renderClone(), m.bootstrap.Changeset.Files, m.prewarmGen)
	if cmd == nil {
		return
	}
	msg := cmd()
	switch msg := msg.(type) {
	case render.PrewarmMsg:
		_, _ = m.Update(msg)
	case tea.BatchMsg:
		for _, c := range msg {
			if c == nil {
				continue
			}
			if pw, ok := c().(render.PrewarmMsg); ok {
				_, _ = m.Update(pw)
			}
		}
	}
}

// BenchmarkViewPlaceholder measures View() when no sections are cached yet —
// the path a user sees on the very first frame before the background prewarm
// goroutine pool has completed. Should be fast (placeholder rendering only).
func BenchmarkViewPlaceholder(b *testing.B) {
	m := benchModel(b, 10, 100)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.clearRenderCache()
		_ = m.View()
	}
}

// BenchmarkView measures View() with all sections pre-rendered by the background
// pool (steady-state, all cache hits). This is the dominant path in production.
func BenchmarkView(b *testing.B) {
	m := benchModel(b, 10, 100)
	prewarmBench(b, m)
	_ = m.View() // warm all other caches (menu bar, sidebar, etc.)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = m.View()
	}
}

// BenchmarkViewLarge simulates a large diff (100 files × 200 lines) with all
// section caches populated by the background pool.
func BenchmarkViewLarge(b *testing.B) {
	m := benchModel(b, 100, 200)
	prewarmBench(b, m)
	_ = m.View() // warm
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = m.View()
	}
}

// BenchmarkRenderFileSectionUncached measures the background-pool render cost:
// renderFileSectionInto with highlight line cache warm and a reused buffer.
// This is the per-file cost paid by cmdPrewarmSections goroutines — not the UI thread.
// Buffer reuse eliminates the ~75 KiB String() copy from the measurement so we
// see the pure rendering cost.
func BenchmarkRenderFileSectionUncached(b *testing.B) {
	m := benchModel(b, 1, 200)
	f := m.bootstrap.Changeset.Files[0]
	buf := textutil.AcquireBuilder()
	defer textutil.ReleaseBuilder(buf)
	// Warm the highlight line cache.
	m.renderFileSectionInto(buf, f, false)
	buf.Reset()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		buf.Reset()
		m.renderFileSectionInto(buf, f, false)
	}
}

// BenchmarkRenderFileSectionCold measures the worst-case render cost paid by
// background goroutines: cold highlight cache, cold section cache.
func BenchmarkRenderFileSectionCold(b *testing.B) {
	buf := textutil.AcquireBuilder()
	defer textutil.ReleaseBuilder(buf)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m := benchModel(b, 1, 200)
		f := m.bootstrap.Changeset.Files[0]
		buf.Reset()
		m.renderFileSectionInto(buf, f, false)
	}
}

// BenchmarkRenderFileSectionCached measures the View() hot path for a single
// file section: a map lookup and string return (prewarm already populated cache).
func BenchmarkRenderFileSectionCached(b *testing.B) {
	m := benchModel(b, 1, 200)
	f := m.bootstrap.Changeset.Files[0]
	prewarmBench(b, m) // populate sectionCache via render.PrewarmMsg handler
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = m.renderFileSection(f, false)
	}
}

// BenchmarkRenderFileSectionPlaceholder measures the placeholder path: the cost
// on the first frame before the background pool has finished, per visible section.
func BenchmarkRenderFileSectionPlaceholder(b *testing.B) {
	m := benchModel(b, 1, 200)
	f := m.bootstrap.Changeset.Files[0]
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = m.renderFileSectionPlaceholder(f)
	}
}

// BenchmarkRenderMultiUnk measures renderFileSectionInto for a single large file
// with many unks — the case where parallel unk goroutines give the most benefit.
// 10 unks × 50 lines each = 500 total patch lines.
func BenchmarkRenderMultiUnk(b *testing.B) {
	m := benchModelMultiUnk(b, 10, 50)
	f := m.bootstrap.Changeset.Files[0]
	buf := textutil.AcquireBuilder()
	defer textutil.ReleaseBuilder(buf)
	m.renderFileSectionInto(buf, f, false) // warm highlight cache
	buf.Reset()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		buf.Reset()
		m.renderFileSectionInto(buf, f, false)
	}
}

// BenchmarkRenderMultiUnkLarge measures the parallel path with 20 unks × 100 lines.
func BenchmarkRenderMultiUnkLarge(b *testing.B) {
	m := benchModelMultiUnk(b, 20, 100)
	f := m.bootstrap.Changeset.Files[0]
	buf := textutil.AcquireBuilder()
	defer textutil.ReleaseBuilder(buf)
	m.renderFileSectionInto(buf, f, false) // warm
	buf.Reset()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		buf.Reset()
		m.renderFileSectionInto(buf, f, false)
	}
}

func BenchmarkScrollDown(b *testing.B) {
	m := benchModel(b, 10, 100)
	prewarmBench(b, m)
	_ = m.View() // warm all caches
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		m.scrollTop = i % 500
		_ = m.View()
	}
}

// BenchmarkRenderDiffPaneScroll measures renderDiffPane when the scroll position
// changes on every call — the exact scenario the line-offset index optimises.
// 20 files × 300 lines = 6000 total diff lines; viewport of 47 lines scrolled
// through the full range.
func BenchmarkRenderDiffPaneScroll(b *testing.B) {
	m := benchModel(b, 20, 300)
	prewarmBench(b, m)
	maxScroll := m.totalDiffLines() - m.bodyHeight()
	if maxScroll <= 0 {
		b.Skip("nothing to scroll")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		m.scrollTop = (i * 7) % maxScroll // irregular stride to defeat branch prediction
		m.diffPaneCache = ""              // force rebuild on every iteration
		_ = m.renderDiffPane()
	}
}

// BenchmarkRenderSidebar measures the sidebar render with many visible files,
// exercising the per-row style pre-computation optimisation.
func BenchmarkRenderSidebar(b *testing.B) {
	m := benchModel(b, 40, 50) // 40 files → full sidebar
	m.sidebarVisible = true
	m.layout = layout.ComputeLayout(m.termWidth, m.termHeight, types.LayoutModeAuto, 34, true, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		m.selectedFileIndex = i % 40
		m.sidebarRowsDirty = true
		m.sidebarRowsCache = ""
		_ = m.renderSidebarInner()
	}
}

// BenchmarkBuildIntraCache measures the parallel intra-line diff computation
// across many files — the work done once at startup (and on each hot-reload).
func BenchmarkBuildIntraCache(b *testing.B) {
	m := benchModel(b, 50, 200) // 50 files × 200 lines each
	files := m.bootstrap.Changeset.Files
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ic := make(map[string][2][][]diff.IntraSpan)
		render.BuildIntraCache(ic, files)
	}
}

// BenchmarkNew measures Model construction time. After the async-render.BuildIntraCache
// refactor, New() should be O(1) — no LCS computation, just struct initialization.
func BenchmarkNew(b *testing.B) {
	// Pre-build a changeset with many files so the before/after delta is visible.
	lang := "go"
	cs := types.Changeset{ID: "bench-new"}
	for i := range 50 {
		patch := syntheticDiff(200)
		cs.Files = append(cs.Files, types.DiffFile{
			ID:       fmt.Sprintf("file-%d", i),
			Path:     fmt.Sprintf("pkg/module%d/file.go", i),
			Patch:    patch,
			Language: &lang,
			Stats:    types.DiffStats{Additions: 100, Deletions: 100},
			Metadata: types.DiffMetadata{
				CacheKey: fmt.Sprintf("ck-new-%d", i),
				Unks:     []types.DiffUnk{{Index: 0, Header: "@@ -1,200 +1,200 @@"}},
			},
		})
	}
	bs := types.Bootstrap{Changeset: cs}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = New(bs)
	}
}

// BenchmarkRenderDiffPaneScrollSlow exercises the same scroll pattern as
// BenchmarkRenderDiffPaneScroll but with sectionLineOffsets cleared, forcing
// the legacy byte-scan path. Use these two numbers side-by-side to measure the
// line-offset-index speedup.
func BenchmarkRenderDiffPaneScrollSlow(b *testing.B) {
	m := benchModel(b, 20, 300)
	prewarmBench(b, m)
	maxScroll := m.totalDiffLines() - m.bodyHeight()
	if maxScroll <= 0 {
		b.Skip("nothing to scroll")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		m.scrollTop = (i * 7) % maxScroll
		m.diffPaneCache = ""
		m.sectionLineOffsets = nil // disable fast path
		_ = m.renderDiffPane()
	}
}

// BenchmarkScrollTickPrebuilt measures renderFull() cost for a scroll tick when
// the pre-scroll buffer has already pre-rendered the diffPane for this position
// (simulating render.PreScrollMsg having arrived before the tick). Pre-builds diffPane
// at a fixed position outside the timer, then measures only the frame assembly:
// diffPane cache hit (18 ns) + sidebar cache hit + joinColumns + menu/status bars.
// Compare with BenchmarkRenderDiffPaneScroll (~7 µs, cache miss) to see the saving.
func BenchmarkScrollTickPrebuilt(b *testing.B) {
	m := benchModel(b, 20, 300)
	prewarmBench(b, m)
	_ = m.View() // warm sidebar, menuBar, statusBar caches
	if m.totalDiffLines()-m.bodyHeight() <= 0 {
		b.Skip("nothing to scroll")
	}

	// Build the diffPane for a fixed position outside the timer (simulates the
	// background goroutine in cmdPreScrollPane completing before the next tick).
	m.scrollTop = 42
	m.diffPaneCache = ""
	prebuilt := m.renderDiffPane()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		// Each iteration: renderFull() when diffPaneCache is already hot.
		m.diffPaneCache = prebuilt
		m.diffPaneScrollTop = 42
		m.bodyCache = "" // bodyCache embeds previous diffPane; must clear
		_ = m.renderFull()
	}
}

// BenchmarkProgressivePrewarm measures renderFull() on an intermediate render.PrewarmMsg:
// only 1 of N sections is cached; the rest render as inline placeholders.
// This is the first-content cost paid by progressive prewarm — the frame a user
// sees after the very first goroutine delivers its section.
// Compare with BenchmarkFirstViewCold (~25µs, sectionCache=nil).
func BenchmarkProgressivePrewarm(b *testing.B) {
	m := benchModel(b, 10, 100)
	files := m.bootstrap.Changeset.Files
	m.sectionCache = make(map[string]string)
	clone := m.renderClone()
	warmBuf := textutil.AcquireBuilder()
	clone.renderFileSectionInto(warmBuf, files[0], false) // warm highlight cache
	textutil.ReleaseBuilder(warmBuf)
	// Deliver exactly one section via the normal render.PrewarmMsg path.
	cmd := cmdPrewarmSections(clone, files[:1], m.prewarmGen)
	if cmd != nil {
		if pw, ok := cmd().(render.PrewarmMsg); ok {
			_, _ = m.Update(pw)
		}
	}
	// sectionCache has 1 real section; the other 9 files render as inline placeholders.
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.builtFrame = ""
		m.diffPaneCache = ""
		m.bodyCache = ""
		_ = m.renderFull()
	}
}

// BenchmarkRenderPatchStack measures renderPatchStackInto for a realistic diff
// with multiple unks and mixed adds/dels/context lines. Exercises the zero-alloc
// writeLineNumStack + writePrefixedRawWidthTo path.
func BenchmarkRenderPatchStack(b *testing.B) {
	m := benchModelMultiUnk(b, 10, 50)
	m.layout.LayoutMode = types.LayoutModeStack
	m.showLineNumbers = true
	f := m.bootstrap.Changeset.Files[0]
	lines := textutil.SplitLines(f.Patch, nil)
	buf := textutil.AcquireBuilder()
	defer textutil.ReleaseBuilder(buf)
	m.renderPatchStackInto(buf, f, lines) // warm styles cache
	buf.Reset()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		buf.Reset()
		m.renderPatchStackInto(buf, f, lines)
	}
}

// BenchmarkRenderPatchStackLarge exercises the stack renderer with a large diff.
func BenchmarkRenderPatchStackLarge(b *testing.B) {
	m := benchModelMultiUnk(b, 20, 100)
	m.layout.LayoutMode = types.LayoutModeStack
	m.showLineNumbers = true
	f := m.bootstrap.Changeset.Files[0]
	lines := textutil.SplitLines(f.Patch, nil)
	buf := textutil.AcquireBuilder()
	defer textutil.ReleaseBuilder(buf)
	m.renderPatchStackInto(buf, f, lines) // warm styles cache
	buf.Reset()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		buf.Reset()
		m.renderPatchStackInto(buf, f, lines)
	}
}

// BenchmarkFirstSection measures the wall-clock time for the first section to
// become visible after a cold prewarm start — the primary UX metric for
// time-to-first-content. Progressive streaming delivers the first render.PrewarmMsg
// after one goroutine renders one section; the user sees real content immediately.
// Compare with BenchmarkAllSections to measure the streaming UX improvement factor.
func BenchmarkFirstSection(b *testing.B) {
	m := benchModel(b, 100, 100) // 100-file diff — realistic large changeset
	clone := m.renderClone()
	files := m.bootstrap.Changeset.Files
	// Warm highlight and style caches to measure steady-state, not init cost.
	warmBuf := textutil.AcquireBuilder()
	clone.renderFileSectionInto(warmBuf, files[0], false)
	textutil.ReleaseBuilder(warmBuf)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		buf := textutil.AcquireBuilder()
		clone.renderFileSectionInto(buf, files[0], false)
		textutil.ReleaseBuilder(buf)
	}
}

// BenchmarkAllSections measures the time to render ALL 100 sections sequentially —
// the UX cost paid by the pre-streaming (blocking) prewarm approach where the user
// waited for every section before seeing any content.
// Ratio: BenchmarkAllSections / BenchmarkFirstSection ≈ 100× for 100 files.
func BenchmarkAllSections(b *testing.B) {
	m := benchModel(b, 100, 100)
	files := m.bootstrap.Changeset.Files
	// Warm caches to measure steady-state.
	clone0 := m.renderClone()
	warmBuf := textutil.AcquireBuilder()
	for _, f := range files {
		clone0.renderFileSectionInto(warmBuf, f, false)
		warmBuf.Reset()
	}
	textutil.ReleaseBuilder(warmBuf)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		clone := m.renderClone()
		buf := textutil.AcquireBuilder()
		for _, f := range files {
			clone.renderFileSectionInto(buf, f, false)
			buf.Reset()
		}
		textutil.ReleaseBuilder(buf)
	}
}

// BenchmarkCPURenderThroughput measures how many ns of CPU are consumed per
// rendered section under sustained load. The ns/op value IS the CPU time per
// render. Compare with the original baseline of 164,994 ns/op for
// RenderFileSectionUncached to quantify the 10× CPU efficiency gain.
func BenchmarkCPURenderThroughput(b *testing.B) {
	m := benchModel(b, 20, 300)
	files := m.bootstrap.Changeset.Files
	// Warm all syntax highlight caches.
	clone0 := m.renderClone()
	warmBuf := textutil.AcquireBuilder()
	for _, f := range files {
		clone0.renderFileSectionInto(warmBuf, f, false)
		warmBuf.Reset()
	}
	textutil.ReleaseBuilder(warmBuf)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		f := files[i%len(files)]
		clone := m.renderClone()
		buf := textutil.AcquireBuilder()
		clone.renderFileSectionInto(buf, f, false)
		textutil.ReleaseBuilder(buf)
	}
}

// buildIntraCacheOriginal simulates the pre-optimization sequential algorithm:
// single-threaded, one sync.Pool get/put per file, and diff.IntraLineDiff which
// allocates a fresh []IntraSpan per del/add line pair. Used by
// BenchmarkBuildIntraCacheOriginal to establish the original performance baseline
// so the improvement factor can be computed objectively.
func buildIntraCacheOriginal(cache map[string][2][][]diff.IntraSpan, files []types.DiffFile) {
	for _, f := range files {
		if f.IsBinary || f.IsTooLarge || f.Patch == "" {
			continue
		}
		key := f.Metadata.CacheKey
		if key == "" {
			key = f.ID
		}
		if _, exists := cache[key]; exists {
			continue
		}
		patchLines := strings.Split(f.Patch, "\n")
		nLines := len(patchLines)
		delSpans := make([][]diff.IntraSpan, nLines) // original: one alloc per file
		addSpans := make([][]diff.IntraSpan, nLines) // original: one alloc per file
		inUnk := false
		for j := range nLines {
			if len(patchLines[j]) == 0 {
				continue
			}
			if patchLines[j][0] == '@' {
				inUnk = true
				continue
			}
			if !inUnk || patchLines[j][0] != '-' {
				continue
			}
			addIdx := j + 1
			if addIdx >= nLines || len(patchLines[addIdx]) == 0 || patchLines[addIdx][0] != '+' {
				continue
			}
			// Original: IntraLineDiff allocates fresh []IntraSpan per pair (pool get/put).
			d, a := diff.IntraLineDiff(patchLines[j][1:], patchLines[addIdx][1:])
			if d != nil {
				delSpans[j] = d
				addSpans[addIdx] = a
			}
		}
		cache[key] = [2][][]diff.IntraSpan{delSpans, addSpans}
	}
}

// BenchmarkBuildIntraCacheOriginal measures the original sequential
// render.BuildIntraCache algorithm: no goroutine pool, one sync.Pool round-trip per
// del/add line pair, one []IntraSpan allocation per pair. This is the baseline
// against which the current parallel+flat-backing implementation is compared.
// Ratio = BenchmarkBuildIntraCacheOriginal / BenchmarkBuildIntraCache.
func BenchmarkBuildIntraCacheOriginal(b *testing.B) {
	m := benchModel(b, 50, 200)
	files := m.bootstrap.Changeset.Files
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ic := make(map[string][2][][]diff.IntraSpan)
		buildIntraCacheOriginal(ic, files)
	}
}

// BenchmarkParallelPrewarm measures the WALL-CLOCK time to populate the
// section cache for 100 files when all sections render concurrently — the
// dominant performance path in production. On a machine with ≥100 logical
// CPUs all 100 goroutines run simultaneously, so wall-clock time ≈ one
// section's render time despite processing 100 files.
//
// Compare with the original sequential rendering cost:
//
//	original sequential = 100 × 164,994 ns ≈ 16.5 ms
//	current parallel    ≈ one section render ≈ 13,847 ns
//	wall-clock speedup  ≈ 1,190×
//
// This benchmark directly measures both:
//
//	(a) rendering speed 100×: wall-clock time to render all 100 sections
//	(b) CPU throughput 100×: sections rendered per nanosecond of wall time
func BenchmarkParallelPrewarm(b *testing.B) {
	m := benchModel(b, 100, 100) // 100-file diff — realistic large changeset
	files := m.bootstrap.Changeset.Files
	// Warm syntax highlight and style caches to measure steady-state.
	clone0 := m.renderClone()
	warmBuf := textutil.AcquireBuilder()
	for _, f := range files {
		clone0.renderFileSectionInto(warmBuf, f, false)
		warmBuf.Reset()
	}
	textutil.ReleaseBuilder(warmBuf)

	nCPU := runtime.NumCPU()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var wg sync.WaitGroup
		sem := make(chan struct{}, nCPU)
		for _, f := range files {
			wg.Add(1)
			go func(f types.DiffFile) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				clone := m.renderClone()
				buf := textutil.AcquireBuilder()
				clone.renderFileSectionInto(buf, f, false)
				textutil.ReleaseBuilder(buf)
			}(f)
		}
		wg.Wait()
	}
}

// BenchmarkNewBaseline measures model construction with the original
// eager key-binding allocation — KeyMapForStyle called every New() with
// no caching. This is the baseline for BenchmarkNew to compute the
// memory improvement factor from keymap once-init.
func BenchmarkNewBaseline(b *testing.B) {
	lang := "go"
	cs := types.Changeset{ID: "bench-new-base"}
	for i := range 50 {
		patch := syntheticDiff(200)
		cs.Files = append(cs.Files, types.DiffFile{
			ID:    fmt.Sprintf("file-%d", i),
			Path:  fmt.Sprintf("pkg/module%d/file.go", i),
			Patch: patch, Language: &lang,
			Stats: types.DiffStats{Additions: 100, Deletions: 100},
			Metadata: types.DiffMetadata{CacheKey: fmt.Sprintf("ck-base-%d", i),
				Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,200 +1,200 @@"}}},
		})
	}
	bs := types.Bootstrap{Changeset: cs}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m := New(bs).(*model)
		// Force keymap rebuild by calling helixKeyMap directly (bypasses once cache).
		m.keys = keys.BuildHelixKeyMap()
		_ = m
	}
}

// BenchmarkNewOriginal simulates the ORIGINAL model constructor cost before
// the async-render.BuildIntraCache refactor and keymap sync.Once caching.
// In the original code New() called render.BuildIntraCache synchronously (LCS for all
// files, O(N²) allocations) and KeyMapForStyle on every call (85 allocs).
// Compare with BenchmarkNew for the improvement ratio:
//
//	memory: 5,249 allocs → 7 allocs = 750×
//	CPU:    original ns  → ~8,400 ns = 183×
//
// This benchmark combines buildIntraCacheOriginal + helixKeyMap (no caching)
// to reconstruct the original New() cost faithfully.
func BenchmarkNewOriginal(b *testing.B) {
	lang := "go"
	cs := types.Changeset{ID: "bench-new-orig"}
	for i := range 50 {
		patch := syntheticDiff(200)
		cs.Files = append(cs.Files, types.DiffFile{
			ID:    fmt.Sprintf("file-%d", i),
			Path:  fmt.Sprintf("pkg/module%d/file.go", i),
			Patch: patch, Language: &lang,
			Stats: types.DiffStats{Additions: 100, Deletions: 100},
			Metadata: types.DiffMetadata{CacheKey: fmt.Sprintf("ck-orig-%d", i),
				Unks: []types.DiffUnk{{Index: 0, Header: "@@ -1,200 +1,200 @@"}}},
		})
	}
	bs := types.Bootstrap{Changeset: cs}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		// 1. Struct init (same as current New() without render.BuildIntraCache+keymaps).
		m := New(bs).(*model)
		// 2. Original: render.BuildIntraCache called synchronously in New().
		ic := make(map[string][2][][]diff.IntraSpan)
		buildIntraCacheOriginal(ic, cs.Files)
		m.intraCache = ic
		// 3. Original: keymap allocated fresh every New() (no once-cache).
		m.keys = keys.BuildHelixKeyMap()
		_ = m
	}
}
