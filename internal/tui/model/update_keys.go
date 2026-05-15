package model

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kpango/unk/internal/loader"
	tuikeys "github.com/kpango/unk/internal/tui/keys"
	"github.com/kpango/unk/internal/tui/styles"
	"github.com/kpango/unk/internal/types"
)

// handleKey routes keyboard events to the appropriate mode handler.
func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		return m.handleKeyHelpMode(msg)
	}
	if m.showKeymapList {
		return m.handleKeyKeymapMode(msg)
	}
	if m.menuOpen {
		return m.handleMenuKey(msg)
	}
	switch m.focusArea {
	case FocusFilter:
		return m.handleKeyFilterMode(msg)
	case FocusSearch:
		return m.handleKeySearchMode(msg)
	case FocusCommand:
		return m.handleKeyCommandMode(msg)
	}
	return m.handleKeyNormal(msg)
}

// handleKeyHelpMode handles keys while the help dialog is open.
// Esc/q closes; ←/→/Tab cycles through sections.
func (m *model) handleKeyHelpMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	const nHelpPages = 3
	switch {
	case key.Matches(msg, m.keys.Quit):
		m.showHelp = false
	case key.Matches(msg, m.keys.ScrollLeft), key.Matches(msg, m.keys.MenuLeft):
		m.helpPage = (m.helpPage - 1 + nHelpPages) % nHelpPages
	case key.Matches(msg, m.keys.ScrollRight), key.Matches(msg, m.keys.MenuRight):
		m.helpPage = (m.helpPage + 1) % nHelpPages
	case msg.Type == tea.KeyTab:
		m.helpPage = (m.helpPage + 1) % nHelpPages
	}
	return m, nil
}

// handleKeyKeymapMode handles keys while the keymap selection overlay is open.
// Up/Down navigate, Enter selects, Esc/q dismisses.
func (m *model) handleKeyKeymapMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		m.showKeymapList = false
	case key.Matches(msg, m.keys.ScrollUp), key.Matches(msg, m.keys.MenuUp):
		m.keymapListIdx = (m.keymapListIdx - 1 + len(tuikeys.KeymapStyles)) % len(tuikeys.KeymapStyles)
	case key.Matches(msg, m.keys.ScrollDown), key.Matches(msg, m.keys.MenuDown):
		m.keymapListIdx = (m.keymapListIdx + 1) % len(tuikeys.KeymapStyles)
	case msg.Type == tea.KeyEnter:
		m.keymapStyle = tuikeys.KeymapStyles[m.keymapListIdx]
		m.keys = tuikeys.ApplyOverrides(tuikeys.KeyMapForStyle(m.keymapStyle), m.bootstrap.KeyBindingOverrides)
		m.showKeymapList = false
	}
	return m, nil
}

// handleKeyFilterMode handles keys while the file filter input has focus.
// Tab returns to files; Esc clears text then exits; other keys update the input.
func (m *model) handleKeyFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.ToggleFocus) {
		m.focusArea = FocusFiles
		m.filter.Blur()
		return m, nil
	}
	if msg.Type == tea.KeyEsc {
		if m.filter.Value() != "" {
			m.filter.SetValue("")
			m.sidebarEntriesDirty = true
			m.markSidebarDirty()
			m.diffPaneCache = ""
			m.bodyCache = ""
			m.scrollTop = 0
			m.selectedFileIndex = max(m.selectedFileIndex, 0)
			return m, nil
		}
		m.focusArea = FocusFiles
		m.filter.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	oldVal := m.filter.Value()
	m.filter, cmd = m.filter.Update(msg)
	if m.filter.Value() != oldVal {
		m.sidebarEntriesDirty = true
		m.markSidebarDirty()
		m.diffPaneCache = ""
		m.bodyCache = ""
		m.scrollTop = 0
		newCount := len(m.visibleFiles())
		if m.selectedFileIndex >= newCount {
			m.selectedFileIndex = max(newCount-1, 0)
		}
	}
	return m, cmd
}

// handleKeySearchMode handles keys while the grep search input has focus.
// Enter commits, Esc clears/exits, other keys update the input with live grep.
func (m *model) handleKeySearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.ToggleFocus) {
		m.focusArea = FocusFiles
		m.search.Blur()
		return m, nil
	}
	if msg.Type == tea.KeyEsc {
		m.search.SetValue("")
		if m.grepQuery != "" {
			m.grepQuery = ""
			m.grepMatchLines = nil
			m.clearRenderCache()
		}
		m.focusArea = FocusFiles
		m.search.Blur()
		return m, nil
	}
	if msg.Type == tea.KeyEnter {
		q := strings.TrimSpace(m.search.Value())
		m.focusArea = FocusFiles
		m.search.Blur()
		if q != m.grepQuery {
			m.grepQuery = q
			m.grepMatchLines = nil
			m.clearRenderCache()
		}
		if q != "" {
			m.computeGrepMatches()
			if len(m.grepMatchLines) > 0 {
				m.grepMatchIdx = 0
				m.scrollTop = max(0, m.grepMatchLines[0]-2)
			}
		}
		return m, nil
	}
	var cmd tea.Cmd
	oldSearch := m.search.Value()
	m.search, cmd = m.search.Update(msg)
	if q := strings.TrimSpace(m.search.Value()); q != strings.TrimSpace(oldSearch) {
		if q != m.grepQuery {
			m.grepQuery = q
			m.grepMatchLines = nil
			m.clearRenderCache()
			if q != "" {
				m.computeGrepMatches()
				if len(m.grepMatchLines) > 0 {
					m.grepMatchIdx = 0
					m.scrollTop = max(0, m.grepMatchLines[0]-2)
				}
			}
		}
	}
	m.statusBarDirty = true
	return m, cmd
}

// handleKeyCommandMode handles keys while the command-mode input has focus.
// Enter executes, Esc cancels, Tab completes, other keys update the input.
func (m *model) handleKeyCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		m.focusArea = FocusFiles
		m.cmdInput.Blur()
		m.cmdInput.SetValue("")
		return m, nil
	}
	if msg.Type == tea.KeyEnter {
		input := m.cmdInput.Value()
		m.cmdInput.SetValue("")
		m.focusArea = FocusFiles
		m.cmdInput.Blur()
		return m, m.executeCommand(input)
	}
	if msg.Type == tea.KeyTab {
		prefix := m.cmdInput.Value()
		matches := tuikeys.CommandCompletions(prefix)
		if len(matches) == 1 {
			m.cmdInput.SetValue(matches[0])
			m.cmdInput.CursorEnd()
		}
		m.statusBarDirty = true
		return m, nil
	}
	var cmd tea.Cmd
	m.cmdInput, cmd = m.cmdInput.Update(msg)
	m.statusBarDirty = true
	return m, cmd
}

// handleKeyNormal handles keys in the default (normal) app mode.
func (m *model) handleKeyNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Suspend):
		return m, tea.Suspend

	case key.Matches(msg, m.keys.MenuOpen):
		if !m.pagerMode {
			m.menuOpen = true
			m.activeMenuID = "file"
			m.menuItemIndex = 0
		}

	case key.Matches(msg, m.keys.Help):
		m.showHelp = true

	case key.Matches(msg, m.keys.FocusFilter):
		m.focusArea = FocusFilter
		m.filter.Focus()

	case key.Matches(msg, m.keys.CommandMode):
		m.focusArea = FocusCommand
		m.cmdInput.SetValue("")
		m.cmdInput.Focus()

	case key.Matches(msg, m.keys.FocusSearch):
		m.focusArea = FocusSearch
		m.search.SetValue(m.grepQuery) // pre-fill with committed query for easy edit
		m.search.Focus()

	case key.Matches(msg, m.keys.SearchNext):
		if len(m.grepMatchLines) > 0 {
			m.grepMatchIdx = (m.grepMatchIdx + 1) % len(m.grepMatchLines)
			m.scrollTop = max(0, m.grepMatchLines[m.grepMatchIdx]-2)
		}

	case key.Matches(msg, m.keys.SearchPrev):
		if len(m.grepMatchLines) > 0 {
			m.grepMatchIdx = (m.grepMatchIdx - 1 + len(m.grepMatchLines)) % len(m.grepMatchLines)
			m.scrollTop = max(0, m.grepMatchLines[m.grepMatchIdx]-2)
		}

	case key.Matches(msg, m.keys.ToggleFocus):
		if m.focusArea == FocusFiles {
			m.focusArea = FocusFilter
			m.filter.Focus()
		} else {
			m.focusArea = FocusFiles
			m.filter.Blur()
		}

	case key.Matches(msg, m.keys.PrevUnk):
		m.navigatePrevUnk()
		m.markSidebarDirty()
		return m, nil

	case key.Matches(msg, m.keys.NextUnk):
		m.navigateNextUnk()
		m.markSidebarDirty()
		return m, nil

	case key.Matches(msg, m.keys.PrevAnnotatedUnk):
		m.navigatePrevAnnotatedUnk()
		m.markSidebarDirty()
		return m, nil

	case key.Matches(msg, m.keys.NextAnnotatedUnk):
		m.navigateNextAnnotatedUnk()
		m.markSidebarDirty()
		return m, nil

	case key.Matches(msg, m.keys.PrevFile):
		m.navigatePrevFile()
		return m, nil

	case key.Matches(msg, m.keys.NextFile):
		m.navigateNextFile()
		return m, nil

	case key.Matches(msg, m.keys.OpenInEditor):
		return m, m.cmdOpenInEditor()

	case key.Matches(msg, m.keys.YankUnk):
		text, err := m.cmdYankUnk()
		if err != nil {
			m.updateNotice = "yank failed: " + err.Error()
		} else if text != "" {
			lines := strings.Count(text, "\n")
			m.updateNotice = fmt.Sprintf("yanked %d lines", lines)
		}
		m.statusBarDirty = true
		return m, nil

	case key.Matches(msg, m.keys.ScrollDown):
		m.stopScrollMomentum()
		maxST := max(0, m.totalDiffLines()-m.bodyHeight())
		m.scrollTop = min(m.scrollTop+1, maxST)
		return m, m.handleKeyScroll(1)

	case key.Matches(msg, m.keys.ScrollUp):
		m.stopScrollMomentum()
		m.scrollTop = max(0, m.scrollTop-1)
		return m, m.handleKeyScroll(-1)

	case key.Matches(msg, m.keys.ScrollDownHalf):
		m.stopScrollMomentum()
		maxST := max(0, m.totalDiffLines()-m.bodyHeight())
		m.scrollTop = min(m.scrollTop+m.layout.TermHeight/2, maxST)

	case key.Matches(msg, m.keys.ScrollUpHalf):
		m.stopScrollMomentum()
		m.scrollTop = max(0, m.scrollTop-m.layout.TermHeight/2)

	case key.Matches(msg, m.keys.ScrollPageDown):
		m.stopScrollMomentum()
		maxST := max(0, m.totalDiffLines()-m.bodyHeight())
		m.scrollTop = min(m.scrollTop+m.layout.TermHeight, maxST)

	case key.Matches(msg, m.keys.ScrollPageUp):
		m.stopScrollMomentum()
		m.scrollTop = max(0, m.scrollTop-m.layout.TermHeight)

	case key.Matches(msg, m.keys.ScrollTop):
		m.stopScrollMomentum()
		m.scrollTop = 0

	case key.Matches(msg, m.keys.ScrollBottom):
		m.stopScrollMomentum()
		m.scrollTop = max(0, m.totalDiffLines()-m.bodyHeight())

	case key.Matches(msg, m.keys.ScrollLeft):
		if prev := m.codeHorizontalOffset; prev > 0 {
			m.codeHorizontalOffset = prev - 1
			m.clearRenderCache()
		}

	case key.Matches(msg, m.keys.ScrollRight):
		m.codeHorizontalOffset++
		m.clearRenderCache()

	case key.Matches(msg, m.keys.ScrollLeftFast):
		if prev := m.codeHorizontalOffset; prev > 0 {
			m.codeHorizontalOffset = max(0, prev-8)
			m.clearRenderCache()
		}

	case key.Matches(msg, m.keys.ScrollRightFast):
		m.codeHorizontalOffset += 8
		m.clearRenderCache()

	case key.Matches(msg, m.keys.LayoutSplit):
		m.setLayoutMode(types.LayoutModeSplit)

	case key.Matches(msg, m.keys.LayoutStack):
		m.setLayoutMode(types.LayoutModeStack)

	case key.Matches(msg, m.keys.LayoutAuto):
		m.setLayoutMode(types.LayoutModeAuto)

	case key.Matches(msg, m.keys.ToggleSidebar):
		m.toggleSidebar()
		m.invalidateFrame()

	case key.Matches(msg, m.keys.ToggleLineNumbers):
		m.showLineNumbers = !m.showLineNumbers
		m.invalidateFrame()

	case key.Matches(msg, m.keys.ToggleWrap):
		m.wrapLines = !m.wrapLines
		m.invalidateFrame()

	case key.Matches(msg, m.keys.ToggleUnkHeaders):
		m.showUnkHeaders = !m.showUnkHeaders
		m.invalidateFrame()
		// Section line counts change when unk headers toggle; re-anchor scrollTop.
		m.scrollTop = m.computeUnkScrollOffset()

	case key.Matches(msg, m.keys.ToggleAgentNotes):
		m.showAgentNotes = !m.showAgentNotes
		m.clearRenderCache()
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		if loader.CanReloadInput(m.bootstrap.Input) {
			return m, m.handleWatchTick()
		}

	case key.Matches(msg, m.keys.CycleTheme):
		m.themeID = styles.NextTheme(m.themeID)
		m.clearRenderCache()
	}

	return m, nil
}
