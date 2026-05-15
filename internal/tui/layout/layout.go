package layout

import (
	"github.com/kpango/unk/internal/types"
	"github.com/mattn/go-runewidth"
)

// Viewport classifies terminal widths into responsive layout tiers.
type Viewport string

const (
	ViewportTight  Viewport = "tight"  // < 160 columns
	ViewportMedium Viewport = "medium" // 160–219 columns
	ViewportFull   Viewport = "full"   // >= 220 columns
)

const (
	viewportTightMax  = 159
	viewportMediumMax = 219

	SidebarMinWidth = 22
	DiffMinWidth    = 48
)

// BoxCharW is the terminal column width of box-drawing characters (│, ─, █).
// These are East Asian Ambiguous: 1 column under en_US, 2 columns under ja_JP.
// go-runewidth reads LC_ALL/LC_CTYPE/LANG at startup; lipgloss always uses 1.
// All layout constants and rendering helpers key off this value so that row
// widths agree with the terminal regardless of locale.
var BoxCharW = max(runewidth.RuneWidth('│'), 1)

// EllipsisW is the terminal column width of "…" (U+2026 HORIZONTAL ELLIPSIS).
// East Asian Ambiguous: 1 column under en_US, 2 columns under ja_JP.
// Used wherever we truncate text and append "…" to reserve the right column count.
var EllipsisW = max(runewidth.RuneWidth('…'), 1)

var (
	DividerWidth         = BoxCharW
	BodyPadding          = BoxCharW
	ForceSidebarMinWidth = SidebarMinWidth + BoxCharW + DiffMinWidth
)

// classifyViewport maps terminal width to a Viewport tier.
func classifyViewport(width int) Viewport {
	if width <= viewportTightMax {
		return ViewportTight
	}
	if width <= viewportMediumMax {
		return ViewportMedium
	}
	return ViewportFull
}

// Layout carries all derived column widths for one render frame.
type Layout struct {
	TermWidth  int
	TermHeight int
	Viewport   Viewport

	// LayoutMode is the resolved (possibly responsive) layout mode.
	LayoutMode types.LayoutMode

	// RenderSidebar is whether the sidebar column is visible this frame.
	RenderSidebar bool
	// SidebarWidth is the actual clamped sidebar column width.
	SidebarWidth int
	// DiffPaneWidth is the usable diff pane column width.
	DiffPaneWidth int
	// DiffContentWidth is DiffPaneWidth minus inner padding.
	DiffContentWidth int
}

// ComputeLayout derives the full column geometry from terminal dimensions
// and current user state.
func ComputeLayout(termWidth, termHeight int, mode types.LayoutMode, sidebarWidth int, sidebarVisible, forceSidebarOpen bool) Layout {
	vp := classifyViewport(termWidth)

	// Resolve whether to actually render the sidebar.
	// Show by default on medium (≥ 160 cols) and full (≥ 220 cols) viewports.
	showSidebar := false
	if vp == ViewportMedium || vp == ViewportFull {
		showSidebar = true
	}
	if forceSidebarOpen && termWidth >= ForceSidebarMinWidth {
		showSidebar = true
	}
	if !sidebarVisible {
		showSidebar = false
	}

	// Resolve responsive layout mode.
	resolvedMode := mode
	if vp == ViewportTight {
		resolvedMode = types.LayoutModeStack
	} else if mode == types.LayoutModeAuto {
		resolvedMode = types.LayoutModeSplit
	}

	bodyWidth := termWidth - BodyPadding
	if bodyWidth < 0 {
		bodyWidth = 0
	}
	availableCenter := bodyWidth
	if showSidebar {
		availableCenter -= DividerWidth
	}

	maxSidebarW := availableCenter - DiffMinWidth
	if maxSidebarW < SidebarMinWidth {
		maxSidebarW = SidebarMinWidth
	}
	clampedSidebar := Clamp(sidebarWidth, SidebarMinWidth, maxSidebarW)

	diffPaneW := availableCenter
	if showSidebar {
		diffPaneW = availableCenter - clampedSidebar
		if diffPaneW < DiffMinWidth {
			diffPaneW = DiffMinWidth
		}
	}
	diffContentW := diffPaneW
	if diffContentW < 12 {
		diffContentW = 12
	}

	return Layout{
		TermWidth:        termWidth,
		TermHeight:       termHeight,
		Viewport:         vp,
		LayoutMode:       resolvedMode,
		RenderSidebar:    showSidebar,
		SidebarWidth:     clampedSidebar,
		DiffPaneWidth:    diffPaneW,
		DiffContentWidth: diffContentW,
	}
}

func Clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }
