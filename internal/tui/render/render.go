package render

import (
	"bytes"
	"runtime"
	"strings"
	"sync"

	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/tui/patch"
	"github.com/kpango/unk/internal/tui/styles"
	"github.com/kpango/unk/internal/tui/textutil"
	"github.com/kpango/unk/internal/types"
)

// MinParallelUnks is the minimum unk count before switching to goroutine-parallel
// rendering. Goroutine wakeup latency on Linux scales with GOMAXPROCS (more Ps →
// more scheduler overhead per futex wake). On machines with many logical CPUs the
// overhead exceeds the parallelism gain for typical file sizes, so we raise the
// threshold to keep sequential rendering dominant for all but the largest diffs.
//
//   - GOMAXPROCS ≥ 64: threshold 64
//   - GOMAXPROCS 8–63: threshold 16
//   - GOMAXPROCS < 8: threshold 4
var MinParallelUnks = func() int {
	n := runtime.GOMAXPROCS(0)
	switch {
	case n >= 64:
		return 64
	case n >= 8:
		return 16
	default:
		return 4
	}
}()

// RenderUnksParallelInto renders unk segments concurrently into out using a bounded
// goroutine pool (min(NumCPU, len(segs)) workers). Each goroutine renders into a
// pool-provided *bytes.Buffer; after all goroutines finish the buffers are copied
// to out in order and returned to the pool. This eliminates one intermediate string
// allocation per segment (vs the old approach of returning strings and joining them).
// renderFn must be safe to call from multiple goroutines simultaneously.
func RenderUnksParallelInto(out *bytes.Buffer, segs []patch.Segment, renderFn func(patch.Segment, *bytes.Buffer)) {
	n := len(segs)
	bufs := make([]*bytes.Buffer, n)
	for i := range bufs {
		bufs[i] = textutil.AcquireBuilder()
	}

	workers := min(runtime.NumCPU(), n)
	sem := make(chan struct{}, workers)

	var wg sync.WaitGroup
	for i, seg := range segs {
		wg.Add(1)
		go func(i int, seg patch.Segment) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			renderFn(seg, bufs[i])
		}(i, seg)
	}
	wg.Wait()

	for _, b := range bufs {
		out.Write(b.Bytes())
		textutil.ReleaseBuilder(b)
	}
}

// PrewarmMsg carries section strings pre-rendered by a background worker pool.
// Gen is matched against Model.prewarmGen in the Update handler; results from a
// superseded generation are discarded rather than polluting sectionCache with
// stale content.
type PrewarmMsg struct {
	Gen      uint64
	Sections map[string]string
}

// PreScrollMsg carries a pre-rendered diffPane string for a predicted scroll
// position. Delivered by cmdPreScrollPane; the Update handler stores it in
// diffPaneCache so the next scrollTickMsg that lands on this position finds an
// O(1) diffPane hit instead of a full O(bodyH × sectionLen) rebuild.
type PreScrollMsg struct {
	Gen       uint64
	ScrollTop int
	DiffPane  string
}

// AgentNoteBox renders a bordered annotation box with the given content width.
func AgentNoteBox(ann types.AgentAnnotation, contentWidth int, rs *styles.RendererStyles) string {
	if contentWidth < 4 {
		return ""
	}
	bw := layout.BoxCharW
	inner := contentWidth - 2*bw // terminal cols inside the │ border chars

	borderStyle := rs.NoteBorder
	titleStyle := rs.NoteTitle
	bodyStyle := rs.NoteBody

	top := borderStyle.Render("┌"+textutil.ColFill('─', inner)+"┐") + "\n"

	titleText := " " + ann.Summary
	if textutil.VisibleWidth(titleText) > inner {
		titleText = textutil.TruncateColumns(titleText, inner-layout.EllipsisW) + "…"
	}
	// Use manual padding (visibleWidth-based) so East Asian Ambiguous chars like "…"
	// are measured correctly. lipgloss.Style.Width() uses uniseg (always 1 col for "…")
	// which would over-pad by 1 under Japanese locale, widening the row past contentWidth.
	titlePadded := titleText + strings.Repeat(" ", max(inner-textutil.VisibleWidth(titleText), 0))
	titleLine := borderStyle.Render("│") + titleStyle.Render(titlePadded) + borderStyle.Render("│") + "\n"

	sb := textutil.AcquireBuilder()
	defer textutil.ReleaseBuilder(sb)
	sb.WriteString(top)
	sb.WriteString(titleLine)

	if ann.Rationale != nil && *ann.Rationale != "" {
		mid := borderStyle.Render("├"+textutil.ColFill('─', inner)+"┤") + "\n"
		rat := " " + *ann.Rationale
		if textutil.VisibleWidth(rat) > inner {
			rat = textutil.TruncateColumns(rat, inner-layout.EllipsisW) + "…"
		}
		ratPadded := rat + strings.Repeat(" ", max(inner-textutil.VisibleWidth(rat), 0))
		bodyLine := borderStyle.Render("│") + bodyStyle.Render(ratPadded) + borderStyle.Render("│") + "\n"
		sb.WriteString(mid)
		sb.WriteString(bodyLine)
	}

	bot := borderStyle.Render("└"+textutil.ColFill('─', inner)+"┘") + "\n"
	sb.WriteString(bot)
	return sb.String()
}
