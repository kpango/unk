package model

import (
	"maps"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/tui/render"
	"github.com/kpango/unk/internal/tui/textutil"
	"github.com/kpango/unk/internal/types"
)

// diffPaneClone builds a minimal Model clone that can run renderDiffPane() in a
// background goroutine at targetScrollTop. Maps are shallow-copied to prevent data
// races with the main goroutine's prewarmMsg handlers, which write to the originals.
func (m *model) diffPaneClone(targetScrollTop int) *model {
	sc := maps.Clone(m.sectionCache)
	lc := maps.Clone(m.sectionLineCache)
	// []int32 slices are immutable once built by buildLineOffsets; only the map
	// itself is copied to avoid races on concurrent map read/write from prewarmMsg.
	lo := maps.Clone(m.sectionLineOffsets)
	return &model{
		layout:               m.layout,
		themeID:              m.themeID,
		showLineNumbers:      m.showLineNumbers,
		wrapLines:            m.wrapLines,
		showUnkHeaders:       m.showUnkHeaders,
		showAgentNotes:       m.showAgentNotes,
		codeHorizontalOffset: m.codeHorizontalOffset,
		grepQuery:            m.grepQuery,
		intraCache:           m.intraCache,
		sectionCache:         sc,
		sectionLineCache:     lc,
		sectionLineOffsets:   lo,
		bootstrap:            m.bootstrap,
		termWidth:            m.termWidth,
		termHeight:           m.termHeight,
		pagerMode:            m.pagerMode,
		filter:               m.filter, // value copy — only Value() is read, safe for concurrent use
		scrollTop:            targetScrollTop,
	}
}

// cmdPreScrollPane pre-renders renderDiffPane() for the scroll position stored
// in clone.scrollTop (set by diffPaneClone), delivering the result as render.PreScrollMsg.
// Call only when sectionCache is fully populated and the animation is expected to
// continue (|nextVelocity| ≥ scrollStopThreshold), so the render is likely to
// be useful by the time the next scrollTickMsg arrives.
func cmdPreScrollPane(clone *model, gen uint64) tea.Cmd {
	return func() tea.Msg {
		diffPane := clone.renderDiffPane()
		return render.PreScrollMsg{Gen: gen, ScrollTop: clone.scrollTop, DiffPane: diffPane}
	}
}

// renderClone returns a minimal *Model snapshot safe for concurrent read-only use
// in background rendering goroutines. Fields that are not needed for
// renderFileSectionInner are left at their zero values.
//
// Safety:
//   - Scalar fields (bool, int, string, struct) are copied by value.
//   - intraCache: the pointer is copied; the underlying map is built once in
//     BuildIntraCache and never modified in-place — only replaced wholesale via
//     watchReloadMsg (a new map pointer). The goroutine holds a reference to the
//     snapshot's map, which remains stable for its lifetime.
//   - liveComments: the pointer is copied. Update handlers use copy-on-write
//     (allocate a new map, then assign), so the snapshot's map is never modified
//     concurrently.
func (m *model) renderClone() *model {
	return &model{
		layout:               m.layout,
		themeID:              m.themeID,
		showLineNumbers:      m.showLineNumbers,
		wrapLines:            m.wrapLines,
		showUnkHeaders:       m.showUnkHeaders,
		showAgentNotes:       m.showAgentNotes,
		codeHorizontalOffset: m.codeHorizontalOffset,
		grepQuery:            m.grepQuery,
		intraCache:           m.intraCache,
		sectionCache:         make(map[string]string),
	}
}

// handlePrewarmMsg merges background-rendered sections into the section cache.
// Discards results from a superseded generation (a later cache invalidation already happened).
// Adjusts scrollTop to account for actual vs. estimated section heights, keeping the
// viewport anchored to the same diff content after sections finish rendering.
func (m *model) handlePrewarmMsg(msg render.PrewarmMsg) {
	if msg.Gen != m.prewarmGen {
		return
	}

	// Pre-compute line offsets for all incoming sections. lineCount = max(len(offsets), 1)
	// matches what sectionLineCache stores, so scrollAdjust uses the same coordinate system
	// as the viewport rendering.
	type sectionMeta struct {
		offsets   []int32
		lineCount int
	}
	batchMeta := make(map[string]sectionMeta, len(msg.Sections))
	for k, v := range msg.Sections {
		off := textutil.BuildLineOffsets(v)
		batchMeta[k] = sectionMeta{offsets: off, lineCount: max(len(off), 1)}
	}

	// Compute scroll adjustment: for each section above the viewport, add the
	// difference between the actual rendered height and the prior estimate.
	scrollAdjust := 0
	lineOffset := 0
	for _, f := range m.visibleFiles() {
		key := m.sectionCacheKey(f)
		oldH := 0
		if lc, ok := m.sectionLineCache[key]; ok {
			oldH = lc
		} else {
			oldH = m.sectionLineCountEstimate(f)
		}
		if meta, inBatch := batchMeta[key]; inBatch {
			if lineOffset+oldH <= m.scrollTop {
				scrollAdjust += meta.lineCount - oldH
			}
		}
		lineOffset += oldH
	}

	// Copy-on-write: replace the cache maps so any goroutine holding the old pointer
	// can finish reading without a data race.
	newCache := make(map[string]string, len(m.sectionCache)+len(msg.Sections))
	maps.Copy(newCache, m.sectionCache)
	maps.Copy(newCache, msg.Sections)
	m.sectionCache = newCache
	newLineCache := make(map[string]int, len(m.sectionLineCache)+len(msg.Sections))
	maps.Copy(newLineCache, m.sectionLineCache)
	newOffsets := make(map[string][]int32, len(m.sectionLineOffsets)+len(msg.Sections))
	maps.Copy(newOffsets, m.sectionLineOffsets)
	for k, meta := range batchMeta {
		newLineCache[k] = meta.lineCount
		newOffsets[k] = meta.offsets
	}
	m.sectionLineCache = newLineCache
	m.sectionLineOffsets = newOffsets

	if scrollAdjust != 0 {
		newTotal := m.totalDiffLines()
		newMax := max(0, newTotal-m.bodyHeight())
		m.scrollTop = layout.Clamp(m.scrollTop+scrollAdjust, 0, newMax)
	}
	m.diffPaneCache = "" // new sections → rebuild diff pane on next View
	m.fileViewDirty = true // fileViewCache must be rebuilt with new lineCount/offsets/rendered data
	if m.grepQuery != "" {
		m.computeGrepMatches()
	}
	if m.prewarmPending > 0 {
		m.prewarmPending--
	}
}

// cmdPrewarmSections renders file sections in a bounded goroutine pool and
// delivers each section as an independent render.PrewarmMsg the moment it finishes.
// Using tea.Batch instead of a single blocking Cmd means sections stream into
// the model one at a time: the first file appears after ~one section's render
// time instead of waiting for all N to complete. For large diffs this reduces
// "time to first real content" from hundreds of milliseconds to single-digit
// milliseconds.
//
// Concurrency is bounded to min(NumCPU, nFiles) via a shared channel semaphore
// captured by all per-file Cmd closures, so CPU usage matches the old approach.
func cmdPrewarmSections(clone *model, files []types.DiffFile, gen uint64) tea.Cmd {
	if len(files) == 0 {
		return nil
	}

	// Skip files already present in the clone's section cache (e.g., from a
	// partial prior prewarm that wasn't superseded by a gen change).
	work := make([]types.DiffFile, 0, len(files))
	keys := make([]string, 0, len(files))
	for _, f := range files {
		key := clone.sectionCacheKey(f)
		if _, ok := clone.sectionCache[key]; !ok {
			work = append(work, f)
			keys = append(keys, key)
		}
	}
	n := len(work)
	if n == 0 {
		return nil
	}

	// Shared semaphore: at most min(NumCPU, n) goroutines render simultaneously.
	sem := make(chan struct{}, min(runtime.NumCPU(), n))

	cmds := make([]tea.Cmd, n)
	for i := range n {
		f, key := work[i], keys[i]
		cmds[i] = func() tea.Msg {
			sem <- struct{}{}
			defer func() { <-sem }()
			buf := textutil.AcquireBuilder()
			clone.renderFileSectionInto(buf, f, false)
			content := buf.String()
			textutil.ReleaseBuilder(buf)
			if clone.grepQuery != "" {
				content = textutil.ApplyGrepHighlightToSection(content, clone.grepQuery)
			}
			return render.PrewarmMsg{Gen: gen, Sections: map[string]string{key: content}}
		}
	}
	return tea.Batch(cmds...)
}
