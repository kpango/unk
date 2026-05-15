package model

import (
	"time"

	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/types"
)

// clearRenderCache drops all pre-rendered strings and schedules a background
// section pre-warp. Call whenever any display state that affects rendering changes.
//
// sectionLineCache (line counts) is cleared here so that computeUnkScrollOffset
// and totalDiffLines use sectionLineCountEstimate (not stale counts from a prior
// layout mode). With accurate estimates (RC 30-33), scrollAdjust == 0 after every
// prewarm in steady state, and the cross-mode inconsistency — where old-layout
// actual counts were mixed with new-layout patchLinesBeforeUnk* offsets — is gone.
//
// sectionLineOffsets (byte indices into sectionCache strings) must also be
// cleared because the strings they reference are being discarded.
func (m *model) clearRenderCache() {
	// nil instead of make(): nil-map reads return zero values safely, so all
	// read sites work unchanged. nil signals "prewarm in-flight" to the sync
	// render guard in Update(), keeping the old builtFrame visible rather than
	// flashing placeholder sections during horizontal scroll / theme changes.
	m.sectionCache = nil
	m.sectionLineCache = nil
	m.sectionLineOffsets = nil
	m.patchLinesCache = nil      // lazy init in renderPatchInto on first write
	m.fileViewDirty = true       // fileViewCache references sectionCache/lineCache/offsets
	m.sidebarEntriesDirty = true // force row string rebuild after theme/size change
	m.sidebarRowsDirty = true
	m.menuBarDirty = true
	m.statusBarDirty = true
	m.divCharCache = ""
	m.cachedPadLine = ""
	m.diffPaneCache = ""
	m.bodyCache = ""
	// builtFrame is intentionally NOT cleared here. The old frame stays visible
	// while background prewarm runs — avoids placeholder flashes. The sync render
	// in Update() is guarded by sectionCache != nil, so it only fires once real
	// sections arrive. Callers that need an immediate frame reset (e.g. window
	// resize, where the old frame has wrong dimensions) must explicitly set
	// m.builtFrame = "" afterward.
	m.grepMatchLines = nil
	m.grepMatchFileSet = nil
	m.grepRegex = nil
	m.prewarmGen++
	m.pendingPrewarm = true
	m.prewarmPending = 0              // reset; outer Update sets it to len(files) when dispatching
	m.lastPrewarmRender = time.Time{} // reset so the first msg of the new cycle always renders
}

// markSidebarDirty invalidates sidebar rows, menu bar, and status bar caches.
// Call on selection change or filter change (visible-file stats and bar content change).
func (m *model) markSidebarDirty() {
	m.sidebarRowsDirty = true
	m.menuBarDirty = true
	m.statusBarDirty = true
}

// setLayoutMode changes the layout mode, recomputes geometry, and clears the
// render cache. The old builtFrame stays visible until prewarm delivers new
// sections — this avoids the loading-screen flash that clearing builtFrame
// would cause. Always call this instead of manually assigning m.layoutMode +
// layout.ComputeLayout + clearRenderCache.
func (m *model) setLayoutMode(mode types.LayoutMode) {
	m.layoutMode = mode
	m.layout = layout.ComputeLayout(m.termWidth, m.termHeight, m.layoutMode, m.sidebarWidth, m.sidebarVisible, m.forceSidebarOpen)
	m.clearRenderCache()
	m.scrollTop = m.computeUnkScrollOffset()
}

// invalidateFrame clears the render cache so prewarm rebuilds section content.
// The old builtFrame stays visible until new sections arrive, avoiding the
// loading-screen flash. Use for display-flag toggles (line numbers, wrap, etc.)
// where only content changes, not terminal geometry.
func (m *model) invalidateFrame() {
	m.clearRenderCache()
}
