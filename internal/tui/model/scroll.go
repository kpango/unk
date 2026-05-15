package model

// scroll.go — kinetic scroll, scrollbar, sidebar-drag, and key-hold scroll.

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kpango/unk/internal/tui/layout"
	tuimsg "github.com/kpango/unk/internal/tui/msg"
	"github.com/kpango/unk/internal/tui/sidebar"
)

// cmdScrollTick schedules the next momentum animation step (~60 fps).
func cmdScrollTick() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(time.Time) tea.Msg {
		return tuimsg.ScrollTick{}
	})
}

// --- scroll momentum constants ---

const (
	scrollFriction     = 0.85 // velocity multiplier applied each 16 ms frame
	scrollWheelImpulse = 3.5  // lines/frame added per wheel notch
	scrollMaxVelocity  = 30.0 // absolute cap to prevent runaway on fast trackpads
	// scrollStopThreshold is raised to 0.5 to stop the momentum animation before
	// it enters the sub-integer velocity zone (v < 1.0 lines/frame) where integer
	// truncation produces alternating 0-and-1-line frames — visible as choppiness.
	// At threshold=0.5 the total coast distance per notch is ~20 lines (same as the
	// old 3.0/0.15 formula) because the higher impulse (3.5) compensates for the
	// earlier stop (3.5/0.15 - 0.5/0.15 ≈ 20 lines). The animation duration is
	// ~205 ms instead of ~334 ms, but every frame produces ≥1 line of movement so
	// it looks and feels consistently smooth.
	scrollStopThreshold = 0.5 // stop ticking when |velocity| drops below this
)

// --- key-scroll (J/K hold) constants ---

const (
	// keyScrollInterval is the tick period while a J/K key is held (≈120 fps).
	keyScrollInterval = 8 * time.Millisecond
	// keyScrollInitialDelay is the watchdog timeout for the very first press.
	// It must exceed the OS initial key-repeat delay (~500 ms on most systems)
	// so the loop stays alive through the gap before repeats begin arriving.
	keyScrollInitialDelay = 650 * time.Millisecond
	// keyScrollRepeatDelay is the watchdog timeout once OS repeats are flowing.
	// Must be larger than the longest expected OS key-repeat interval (~125 ms
	// for 8/s, the slowest common setting). 200 ms gives a 60 % safety margin
	// and means at most ~12 lines of smooth coasting after key release.
	keyScrollRepeatDelay = 200 * time.Millisecond
)

// --- mouse handling ---

func (m *model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	var tickCmd tea.Cmd
	switch msg.Action {
	case tea.MouseActionPress:
		switch msg.Button {
		case tea.MouseButtonWheelDown:
			m.scrollVelocity = min(scrollMaxVelocity, m.scrollVelocity+scrollWheelImpulse)
			tickCmd = m.ensureScrollTick()
		case tea.MouseButtonWheelUp:
			m.scrollVelocity = max(-scrollMaxVelocity, m.scrollVelocity-scrollWheelImpulse)
			tickCmd = m.ensureScrollTick()
		case tea.MouseButtonLeft:
			// Scrollbar click: occupies the rightmost layout.BodyPadding terminal columns.
			// Under East Asian locale layout.BodyPadding=2 (wide box chars), so match both cols.
			if m.termWidth > 0 && msg.X >= m.termWidth-layout.BodyPadding {
				m.handleScrollbarPress(msg.Y)
				break
			}
			// Begin sidebar drag if the click lands on the divider column.
			if m.layout.RenderSidebar && msg.X == m.layout.SidebarWidth {
				m.sidebarDragging = true
				m.sidebarDragAnchorX = msg.X
				m.sidebarDragStartW = m.sidebarWidth
				break
			}
			// Sidebar file click: clicking a file row jumps the diff pane to that file.
			if m.layout.RenderSidebar && msg.X < m.layout.SidebarWidth {
				m.stopScrollMomentum()
				menuBarRows := 0
				if !m.pagerMode {
					menuBarRows = 1
				}
				rowInSidebar := msg.Y - menuBarRows
				files := m.visibleFiles()
				entries := sidebar.BuildEntries(nil, files)
				bodyH := m.bodyHeight()
				if rowInSidebar >= 0 && rowInSidebar < bodyH {
					scrollTop := sidebar.ScrollTop(entries, m.selectedFileIndex, bodyH)
					entryIdx := scrollTop + rowInSidebar
					if entryIdx >= 0 && entryIdx < len(entries) {
						e := entries[entryIdx]
						if !e.IsGroup {
							m.selectedFileIndex = e.FileIdx
							m.selectedUnkIndex = 0
							m.scrollTop = m.computeUnkScrollOffset()
							m.markSidebarDirty()
						}
					}
				}
			}
		}
	case tea.MouseActionMotion:
		if m.scrollbarDragging {
			m.handleScrollbarDrag(msg.Y)
			break
		}
		if m.sidebarDragging {
			delta := msg.X - m.sidebarDragAnchorX
			m.sidebarWidth = m.sidebarDragStartW + delta
			m.layout = layout.ComputeLayout(m.termWidth, m.termHeight, m.layoutMode, m.sidebarWidth, m.sidebarVisible, m.forceSidebarOpen)
			// Width changed — cached sections have the wrong line length, drop them.
			// Sidebar drag changes the divider position visually: clear builtFrame so
			// View() shows the new layout immediately rather than the stale old position.
			m.invalidateFrame()
		}
	case tea.MouseActionRelease:
		if m.scrollbarDragging {
			m.scrollbarDragging = false
			break
		}
		if m.sidebarDragging {
			m.sidebarDragging = false
		}
	}
	return m, tickCmd
}

// --- scrollbar helpers ---

// bodyStartRow returns the first terminal row used by the body area.
// Row 0 is the menu bar in normal mode; in pager mode the body starts at row 0.
func (m *model) bodyStartRow() int {
	if m.pagerMode {
		return 0
	}
	return 1
}

// handleScrollbarPress handles a left-click on the scrollbar column.
// Clicking the thumb begins a drag; clicking the track jumps proportionally.
func (m *model) handleScrollbarPress(y int) {
	g, ok := m.scrollbarGeom()
	if !ok {
		return
	}
	rowInBody := y - m.bodyStartRow()
	if rowInBody < 0 || rowInBody >= g.TrackLen {
		return
	}
	// Click on thumb → start drag; stop any in-flight momentum first.
	if rowInBody >= g.ThumbTop && rowInBody < g.ThumbTop+g.ThumbH {
		m.stopScrollMomentum()
		m.scrollbarDragging = true
		m.scrollbarDragAnchorY = y
		m.scrollbarDragScrollTop = m.scrollTop
		return
	}
	// Click on track → jump to that proportion of the content.
	trackFree := g.TrackLen - g.ThumbH
	if trackFree > 0 {
		m.scrollTop = layout.Clamp(rowInBody*g.MaxScroll/trackFree, 0, g.MaxScroll)
	}
	m.markSidebarDirty()
}

// handleScrollbarDrag updates scrollTop to follow the dragged thumb.
func (m *model) handleScrollbarDrag(y int) {
	g, ok := m.scrollbarGeom()
	if !ok {
		m.scrollbarDragging = false
		return
	}
	trackFree := g.TrackLen - g.ThumbH
	if trackFree <= 0 {
		return
	}
	delta := y - m.scrollbarDragAnchorY
	m.scrollTop = layout.Clamp(m.scrollbarDragScrollTop+delta*g.MaxScroll/trackFree, 0, g.MaxScroll)
	m.markSidebarDirty()
}

// --- sidebar toggle ---

// toggleSidebar cycles the sidebar: on tight viewports (sidebar not auto-shown),
// the first toggle forces it open; subsequent toggles hide it.
func (m *model) toggleSidebar() {
	autoShown := m.layout.RenderSidebar && !m.forceSidebarOpen
	canForce := m.termWidth >= layout.ForceSidebarMinWidth

	if m.sidebarVisible && (autoShown || m.forceSidebarOpen) {
		// Sidebar currently visible — hide it.
		m.sidebarVisible = false
		m.forceSidebarOpen = false
	} else if m.sidebarVisible && !autoShown && canForce {
		// Visible but not shown (viewport too narrow) — force it open.
		m.forceSidebarOpen = true
	} else {
		// Hidden — show it; force open if the viewport won't auto-show it.
		m.sidebarVisible = true
		m.forceSidebarOpen = !m.layout.RenderSidebar && canForce
	}
	m.layout = layout.ComputeLayout(m.termWidth, m.termHeight, m.layoutMode, m.sidebarWidth, m.sidebarVisible, m.forceSidebarOpen)
}

// --- key-scroll state machine ---

// handleKeyScroll manages the J/K hold-scroll state machine.
//
// Phase transitions:
//
//	0 → 1  (first press or direction change): arm the long watchdog only.
//	        No tick loop yet — a single tap must not over-scroll.
//	1 → 2  (first OS repeat, same dir): start the 120 fps tick loop.
//	        The long watchdog is superseded by a short one.
//	2       (subsequent OS repeats): keep tick running, refresh short watchdog.
//
// The separate keyScrollWdEpoch counter ensures only the LATEST watchdog can
// stop the loop; stale watchdogs from earlier key events are ignored.
func (m *model) handleKeyScroll(dir int) tea.Cmd {
	m.keyScrollWdEpoch++
	epoch := m.keyScrollWdEpoch

	switch {
	case m.keyScrollPhase == 0 || m.keyScrollDir != dir:
		// First press or direction change: reset, arm long watchdog only.
		// The tick loop is NOT started here so a single tap scrolls exactly 1 line.
		m.keyScrollPhase = 1
		m.keyScrollDir = dir
		return func() tea.Msg {
			time.Sleep(keyScrollInitialDelay)
			return tuimsg.KeyScrollEnd{Epoch: epoch}
		}

	case m.keyScrollPhase == 1:
		// First OS repeat confirms the key is held: start the tick loop.
		m.keyScrollPhase = 2
		m.keyScrollGen++
		gen := m.keyScrollGen
		return tea.Batch(
			func() tea.Msg {
				time.Sleep(keyScrollInterval)
				return tuimsg.KeyScrollTick{Gen: gen, Dir: dir}
			},
			func() tea.Msg {
				time.Sleep(keyScrollRepeatDelay)
				return tuimsg.KeyScrollEnd{Epoch: epoch}
			},
		)

	default:
		// Subsequent OS repeats: tick is already running, just refresh watchdog.
		return func() tea.Msg {
			time.Sleep(keyScrollRepeatDelay)
			return tuimsg.KeyScrollEnd{Epoch: epoch}
		}
	}
}

// ensureScrollTick starts the 16 ms animation ticker if it is not already
// running. Only one tick cmd is ever in flight at a time.
func (m *model) ensureScrollTick() tea.Cmd {
	if m.scrollTicking {
		return nil
	}
	m.scrollTicking = true
	return cmdScrollTick()
}

// stopScrollMomentum cancels any in-progress kinetic scroll. Call this before
// any direct-manipulation action (keyboard scroll, scrollbar drag, file click)
// so that explicit intent immediately wins over lingering inertia.
func (m *model) stopScrollMomentum() {
	m.scrollVelocity = 0
	m.scrollFrac = 0
	// scrollTicking stays true if a tick is already in flight; that tick will see
	// zero velocity and exit naturally without re-arming.
}
