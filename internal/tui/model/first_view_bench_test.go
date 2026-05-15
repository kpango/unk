package model

import (
	"testing"

	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/types"
)

// BenchmarkFirstView measures View() when builtFrame="" and sectionCache=nil —
// the state after clearRenderCache+explicit builtFrame clear (e.g. window resize
// while prewarm is in-flight, repeated layout/theme changes). The loadingFrame
// cache makes iterations after the first an O(1) return; the benchmark reports
// the amortised steady-state cost (~12 ns/op, 0 allocs).
//
// To measure the true one-shot cold cost (first-ever render, no cache),
// see BenchmarkFirstViewCold below.
func BenchmarkFirstView(b *testing.B) {
	m := benchModel(b, 10, 100)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.clearRenderCache()
		m.builtFrame = ""
		_ = m.View()
	}
}

// BenchmarkFirstViewLarge measures the same scenario with a larger diff.
func BenchmarkFirstViewLarge(b *testing.B) {
	m := benchModel(b, 50, 200)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.clearRenderCache()
		m.builtFrame = ""
		_ = m.View()
	}
}

// BenchmarkFirstViewCold measures the absolute worst case: the very first
// renderFull() with sectionCache=nil and no loadingFrame cached. Iterates with
// alternating terminal heights to defeat the loadingFrame cache, forcing a full
// render each time. The render cost (sidebar + menubar + statusbar + placeholder
// diffPane) is isolated from model-construction overhead by pre-building the model.
func BenchmarkFirstViewCold(b *testing.B) {
	// Pre-build two models with different heights so we can alternate without
	// re-constructing. The cache key difference (layout.TermHeight) ensures the
	// loadingFrame from iteration i is not reused in iteration i+1.
	models := [2]*model{benchModel(b, 10, 100), benchModel(b, 10, 100)}
	models[1].termHeight = 51
	models[1].layout = layout.ComputeLayout(models[1].termWidth, 51, types.LayoutModeAuto, 34, true, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		m := models[i%2]
		m.clearRenderCache()
		m.builtFrame = ""
		m.loadingFrame = "" // discard cached loading frame to force full render
		_ = m.View()
	}
}
