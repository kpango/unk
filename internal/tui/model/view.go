package model

import (
	"github.com/kpango/unk/internal/tui/patch"
	"github.com/kpango/unk/internal/types"
)

// fileSectionLineCount delegates to patch.FileSectionLineCount so that callers
// in this package don't need to be updated.
func fileSectionLineCount(f types.DiffFile) int {
	return patch.FileSectionLineCount(f)
}

// View satisfies tea.Model. It returns the latest frame assembled by the Render
// in O(1). Update() pre-builds builtFrame synchronously on every event so this
// is almost always a string return with zero extra work.
//
// Exception: FocusFilter/Search/Command modes with a live blinking cursor
// must re-render synchronously so the cursor blink animation is not frozen.
func (m *model) View() string {
	if m.termWidth == 0 {
		return ""
	}
	liveCursor := m.focusArea == FocusFilter || m.focusArea == FocusSearch || m.focusArea == FocusCommand
	if m.builtFrame != "" && !liveCursor {
		return m.builtFrame
	}
	// Loading-frame fast path: when sectionCache is nil (prewarm in-flight) and
	// builtFrame has been cleared (e.g. window resize, layout toggle), return the
	// pre-built placeholder frame rather than re-rendering from scratch.
	if !liveCursor && m.sectionCache == nil &&
		m.loadingFrame != "" &&
		m.loadingFrameLayout == m.layout &&
		m.loadingFrameTheme == m.themeID &&
		m.loadingFrameCV == m.contentVersion {
		m.builtFrame = m.loadingFrame
		return m.loadingFrame
	}
	// First frame or live-cursor: render inline. Cache the result when stable.
	frame := m.renderFull()
	if !liveCursor {
		m.builtFrame = frame
		if m.sectionCache == nil {
			m.loadingFrame = frame
			m.loadingFrameLayout = m.layout
			m.loadingFrameTheme = m.themeID
			m.loadingFrameCV = m.contentVersion
		}
	}
	return frame
}

// bodyHeight returns the usable vertical lines for the diff/body area.
// Non-pager: 1 menu bar + 1 status bar + 1 bottom margin = -3.
// Pager: no menu bar, no status bar; 1 bottom margin = -1.
func (m *model) bodyHeight() int {
	if m.pagerMode {
		return m.layout.TermHeight - 1
	}
	return m.layout.TermHeight - 3
}
