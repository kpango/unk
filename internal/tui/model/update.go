package model

import (
	"math"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kpango/unk/internal/diff"
	"github.com/kpango/unk/internal/loader"
	"github.com/kpango/unk/internal/tui/layout"
	tuimsg "github.com/kpango/unk/internal/tui/msg"
	"github.com/kpango/unk/internal/tui/render"
	"github.com/kpango/unk/internal/tui/updatecheck"
	"github.com/kpango/unk/internal/types"
)

// -- tea.Model interface --

// Update satisfies tea.Model. It delegates to handleUpdate, then:
//  1. Dispatches background section pre-render (cmdPrewarmSections) when the
//     render cache was invalidated (pendingPrewarm flag set by clearRenderCache).
//  2. Rebuilds builtFrame synchronously so View() returns the updated frame in
//     the SAME BubbleTea cycle as the triggering event, with zero round-trip
//     latency. Typical cost is 10–50 µs (dominated by renderDiffPane), which is
//     well within the BubbleTea frame budget.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.handleUpdate(msg)
	m2 := next.(*model)

	var cmds []tea.Cmd
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Data Thread: section pre-render (syntax highlight, layout).
	if m2.pendingPrewarm {
		m2.pendingPrewarm = false
		files := m2.bootstrap.Changeset.Files
		prewarmCmd := cmdPrewarmSections(m2.renderClone(), files, m2.prewarmGen)
		if prewarmCmd != nil {
			// One render.PrewarmMsg arrives per file via tea.Batch. Track the count so the
			// sync render fires only after ALL sections have been received — not after
			// each intermediate delivery (which would show placeholder rows for files
			// not yet rendered, appearing as progressive corruption to the user).
			m2.prewarmPending = len(files)
			cmds = append(cmds, prewarmCmd)
		} else {
			// No files to render: transition sectionCache from nil to empty so the
			// render guard below passes and renderFull() shows "No changes to review."
			m2.sectionCache = make(map[string]string)
		}
	}

	_, isPrewarmMsg := msg.(render.PrewarmMsg)
	_, isScrollTick := msg.(tuimsg.ScrollTick)
	_, isPreScrollMsg := msg.(render.PreScrollMsg)

	// Skip renderFull() for scroll ticks that didn't move the viewport — the
	// existing builtFrame is still correct and re-rendering wastes ~20 µs per tick.
	scrollUnmoved := isScrollTick && m2.diffPaneCache != "" && m2.scrollTop == m2.diffPaneScrollTop

	// render.PreScrollMsg pre-populates diffPaneCache for the next predicted scroll
	// position without advancing scrollTop. Running renderFull() here would embed
	// the wrong scroll position into builtFrame, so bail early — the current
	// builtFrame remains valid and the next scrollTickMsg will find diffPaneCache
	// already hot (O(1) hit instead of O(bodyH × lines)).
	if isPreScrollMsg {
		return m2, tea.Batch(cmds...)
	}

	// Progressive prewarm: render on intermediate prewarmMsgs so users see the
	// first real file section as soon as any goroutine delivers it, rather than
	// waiting for ALL N sections before showing anything. Rate-limited to one
	// render per 16ms to prevent CPU thrashing when all goroutines finish at once
	// (common on high-CPU machines where section render time < goroutine spawn cost).
	// The final delivery (prewarmPending == 0) always renders unconditionally.
	intermediatePrewarm := isPrewarmMsg && m2.prewarmPending > 0
	skipRateLimit := intermediatePrewarm && time.Since(m2.lastPrewarmRender) < 16*time.Millisecond

	if m2.sectionCache != nil && !scrollUnmoved && !skipRateLimit {
		m2.builtFrame = m2.renderFull()
		if intermediatePrewarm {
			m2.lastPrewarmRender = time.Now()
		}
	}

	// Pre-buffer: after a scroll render that moved the viewport, dispatch a
	// goroutine to pre-render the diffPane for the next predicted scroll position.
	// The result arrives as render.PreScrollMsg and pre-populates diffPaneCache before
	// the next scrollTickMsg, dropping that tick's renderFull cost from ~10µs to ~3µs.
	if isScrollTick && !scrollUnmoved && m2.sectionCache != nil {
		nextVel := m2.scrollVelocity * scrollFriction
		if math.Abs(nextVel) >= scrollStopThreshold {
			nextFrac := m2.scrollFrac + nextVel
			nextScrollTop := layout.Clamp(m2.scrollTop+int(nextFrac), 0, max(0, m2.totalDiffLines()-m2.bodyHeight()))
			if nextScrollTop != m2.scrollTop {
				cmds = append(cmds, cmdPreScrollPane(m2.diffPaneClone(nextScrollTop), m2.prewarmGen))
			}
		}
	}

	return m2, tea.Batch(cmds...)
}

func (m *model) handleUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		oldMode := m.layout.LayoutMode
		m.layout = layout.ComputeLayout(m.termWidth, m.termHeight, m.layoutMode, m.sidebarWidth, m.sidebarVisible, m.forceSidebarOpen)
		m.clearRenderCache()
		// Window resize changes frame dimensions: the old builtFrame has the
		// wrong width/height and would render incorrectly in the new terminal
		// size. Force an immediate synchronous render via View() fallback.
		m.builtFrame = ""
		// If the resolved layout mode changed (e.g. resize crosses the ViewportTight
		// boundary, toggling forced-stack mode), re-anchor scrollTop to the selected
		// unk using the new mode's line-count estimates. Without this, the old
		// scrollTop (expressed in the previous mode's coordinate system) points to
		// the wrong line after the mode change.
		if m.layout.LayoutMode != oldMode {
			m.scrollTop = m.computeUnkScrollOffset()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case render.PrewarmMsg:
		m.handlePrewarmMsg(msg)
		return m, nil

	case tuimsg.ChangesetLoaded:
		if msg.Err != nil {
			m.isLoading = false
			m.loadErr = msg.Err.Error()
			m.clearRenderCache()
			return m, nil
		}
		m.isLoading = false
		m.loadErr = ""
		m.bootstrap = *msg.Bootstrap
		// Re-derive watch mode from the fully-loaded input.
		m.watchEnabled = func() bool {
			opts := types.OptionsOf(msg.Bootstrap.Input)
			return opts.Watch != nil && *opts.Watch && loader.CanReloadInput(msg.Bootstrap.Input)
		}()
		m.contentVersion++
		m.clearRenderCache()
		return m, m.cmdBuildIntraCache()

	case tuimsg.IntraCacheReady:
		// Async intra-line diff build completed. Assign the cache and trigger a
		// second prewarm so all sections are re-rendered with intra highlights.
		m.intraCache = map[string][2][][]diff.IntraSpan(msg)
		m.clearRenderCache()
		return m, nil

	case tuimsg.WatchReload:
		// Hot-reload: swap the changeset. The intra-line diff cache was pre-built
		// in the background goroutine that produced this message.
		m.stopScrollMomentum()
		m.bootstrap.Changeset = msg.Changeset
		m.intraCache = msg.IntraCache
		m.splitSavingsCache = nil // file keys may have changed; invalidate estimate cache
		m.contentVersion++
		m.clearRenderCache()
		files := m.visibleFiles()
		if m.selectedFileIndex >= len(files) {
			m.selectedFileIndex = max(0, len(files)-1)
			m.selectedUnkIndex = 0
			m.scrollTop = 0
		} else if len(files) > 0 {
			unks := files[m.selectedFileIndex].Metadata.Unks
			if m.selectedUnkIndex >= len(unks) {
				m.selectedUnkIndex = max(0, len(unks)-1)
			}
		}
		return m, m.startWatchPoller()

	case tuimsg.WatchTick:
		return m, m.handleWatchTick()

	case tuimsg.ScrollTick:
		m.scrollTicking = false
		// Decay velocity first; this frame displays the result of the previous impulse.
		m.scrollVelocity *= scrollFriction
		m.scrollFrac += m.scrollVelocity
		delta := int(m.scrollFrac)
		m.scrollFrac -= float64(delta)
		if delta != 0 {
			total := m.totalDiffLines()
			bodyH := m.bodyHeight()
			maxScroll := max(0, total-bodyH)
			prev := m.scrollTop
			m.scrollTop = layout.Clamp(m.scrollTop+delta, 0, maxScroll)
			// Hit a boundary — stop immediately instead of coasting in place.
			if m.scrollTop == prev && delta != 0 {
				m.scrollVelocity = 0
				m.scrollFrac = 0
				return m, nil
			}
		}
		if math.Abs(m.scrollVelocity) >= scrollStopThreshold || math.Abs(m.scrollFrac) >= 0.5 {
			return m, m.ensureScrollTick()
		}
		m.scrollVelocity = 0
		m.scrollFrac = 0
		return m, nil

	case tuimsg.IPC:
		followUp := m.handleIPCCmd(msg.Cmd)
		var cmds []tea.Cmd
		if followUp != nil {
			cmds = append(cmds, followUp)
		}
		cmds = append(cmds, m.cmdWaitIPC())
		return m, tea.Batch(cmds...)

	case updatecheck.NoticeMsg:
		m.updateNotice = string(msg)
		m.statusBarDirty = true

	case render.PreScrollMsg:
		// Pre-rendered diffPane for a predicted scroll position. Store it in
		// diffPaneCache so the next ScrollTick that lands on this position
		// gets an O(1) diffPane render instead of a full O(bodyH) rebuild.
		// bodyCache is cleared because it embeds the old diffPane — a stale body
		// returned on the next tick would show the wrong scroll position.
		if msg.Gen == m.prewarmGen && m.sectionCache != nil {
			m.diffPaneCache = msg.DiffPane
			m.diffPaneScrollTop = msg.ScrollTop
			m.bodyCache = ""
		}

	case tuimsg.KeyScrollTick:
		// Stale ticks from a superseded press are ignored.
		if msg.Gen != m.keyScrollGen {
			return m, nil
		}
		maxST := max(0, m.totalDiffLines()-m.bodyHeight())
		prev := m.scrollTop
		m.scrollTop = layout.Clamp(m.scrollTop+msg.Dir, 0, maxST)
		// Hit a boundary — stop the loop.
		if m.scrollTop == prev {
			m.keyScrollGen++
			m.keyScrollPhase = 0
			return m, nil
		}
		// Re-arm the tick for the next frame; the watchdog timer is already running.
		gen, dir := msg.Gen, msg.Dir
		return m, func() tea.Msg {
			time.Sleep(keyScrollInterval)
			return tuimsg.KeyScrollTick{Gen: gen, Dir: dir}
		}

	case tuimsg.KeyScrollEnd:
		// Watchdog fired. Only the latest watchdog (highest epoch) stops the loop;
		// stale watchdogs from superseded key events carry an old epoch and are
		// ignored, preventing premature stops when OS repeats arrive slowly.
		if msg.Epoch == m.keyScrollWdEpoch {
			m.keyScrollGen++
			m.keyScrollPhase = 0
		}

	case tuimsg.EditorFinished:
		// After the external editor exits, trigger a reload if watch mode is enabled
		// so any edits made in the editor are immediately reflected in the diff.
		if m.watchEnabled {
			return m, m.handleWatchTick()
		}
		return m, nil
	}

	// Propagate to filter input when it has focus.
	if m.focusArea == FocusFilter {
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		return m, cmd
	}

	return m, nil
}
