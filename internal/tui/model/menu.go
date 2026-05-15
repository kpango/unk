package model

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kpango/unk/internal/loader"
	"github.com/kpango/unk/internal/tui/styles"
	"github.com/kpango/unk/internal/types"
)

// menuEntry is one item in a dropdown menu.
type menuEntry struct {
	label     string
	hint      string    // keyboard shortcut shown on the right
	checked   *bool     // non-nil → show checkbox; true → checked
	separator bool      // if true, render as a divider line
	action    func(*model) tea.Cmd
}

// menuIDs is the ordered list of top-level menu categories.
var menuIDs = []string{"file", "view", "navigate"}

// buildMenuEntries returns the menu items for the given category and current model state.
func buildMenuEntries(m *model) map[string][]menuEntry {
	canRefresh := loader.CanReloadInput(m.bootstrap.Input)

	fileMenu := []menuEntry{
		{label: "Toggle files/filter focus", hint: "Tab", action: func(m *model) tea.Cmd {
			if m.focusArea == FocusFiles {
				m.focusArea = FocusFilter
				m.filter.Focus()
			} else {
				m.focusArea = FocusFiles
				m.filter.Blur()
			}
			return nil
		}},
		{label: "Focus filter", hint: "/", action: func(m *model) tea.Cmd {
			m.focusArea = FocusFilter
			m.filter.Focus()
			return nil
		}},
	}
	if canRefresh {
		fileMenu = append(fileMenu, menuEntry{label: "Reload", hint: "r", action: func(m *model) tea.Cmd {
			return m.handleWatchTick()
		}})
	}
	fileMenu = append(fileMenu,
		menuEntry{separator: true},
		menuEntry{label: "Open in editor", hint: "o", action: func(m *model) tea.Cmd {
			return m.cmdOpenInEditor()
		}},
		menuEntry{separator: true},
		menuEntry{label: "Quit", hint: "q", action: func(m *model) tea.Cmd { return tea.Quit }},
	)

	t := m.showLineNumbers
	wt := m.wrapLines
	ht := m.showUnkHeaders
	at := m.showAgentNotes
	sb := m.sidebarVisible
	isSplit := m.layoutMode == types.LayoutModeSplit
	isStack := m.layoutMode == types.LayoutModeStack
	isAuto := m.layoutMode == types.LayoutModeAuto || (!isSplit && !isStack)

	viewMenu := []menuEntry{
		{label: "Split view", hint: "1", checked: types.Ptr(isSplit), action: func(m *model) tea.Cmd {
			m.setLayoutMode(types.LayoutModeSplit)
			return nil
		}},
		{label: "Stacked view", hint: "2", checked: types.Ptr(isStack), action: func(m *model) tea.Cmd {
			m.setLayoutMode(types.LayoutModeStack)
			return nil
		}},
		{label: "Auto layout", hint: "0", checked: types.Ptr(isAuto), action: func(m *model) tea.Cmd {
			m.setLayoutMode(types.LayoutModeAuto)
			return nil
		}},
		{separator: true},
		{label: "Sidebar", hint: "s", checked: types.Ptr(sb), action: func(m *model) tea.Cmd {
			m.toggleSidebar()
			m.invalidateFrame()
			return nil
		}},
		{separator: true},
		{label: "Agent notes", hint: "a", checked: types.Ptr(at), action: func(m *model) tea.Cmd {
			m.showAgentNotes = !m.showAgentNotes
			m.clearRenderCache()
			return nil
		}},
		{label: "Line numbers", hint: "l", checked: types.Ptr(t), action: func(m *model) tea.Cmd {
			m.showLineNumbers = !m.showLineNumbers
			m.invalidateFrame()
			return nil
		}},
		{label: "Wrap lines", hint: "w", checked: types.Ptr(wt), action: func(m *model) tea.Cmd {
			m.wrapLines = !m.wrapLines
			m.invalidateFrame()
			return nil
		}},
		{label: "Unk headers", hint: "m", checked: types.Ptr(ht), action: func(m *model) tea.Cmd {
			m.showUnkHeaders = !m.showUnkHeaders
			m.invalidateFrame()
			m.scrollTop = m.computeUnkScrollOffset()
			return nil
		}},
		{separator: true},
		{label: "Cycle theme", hint: "t", action: func(m *model) tea.Cmd {
			m.themeID = styles.NextTheme(m.themeID)
			m.clearRenderCache()
			return nil
		}},
	}

	navMenu := []menuEntry{
		{label: "Previous file", hint: "(", action: func(m *model) tea.Cmd { m.navigatePrevFile(); return nil }},
		{label: "Next file", hint: ")", action: func(m *model) tea.Cmd { m.navigateNextFile(); return nil }},
		{separator: true},
		{label: "Previous unk", hint: "[", action: func(m *model) tea.Cmd { m.navigatePrevUnk(); m.markSidebarDirty(); return nil }},
		{label: "Next unk", hint: "]", action: func(m *model) tea.Cmd { m.navigateNextUnk(); m.markSidebarDirty(); return nil }},
		{separator: true},
		{label: "Prev annotated unk", hint: "{", action: func(m *model) tea.Cmd { m.navigatePrevAnnotatedUnk(); m.markSidebarDirty(); return nil }},
		{label: "Next annotated unk", hint: "}", action: func(m *model) tea.Cmd { m.navigateNextAnnotatedUnk(); m.markSidebarDirty(); return nil }},
		{separator: true},
		{label: "Help", hint: "?", action: func(m *model) tea.Cmd { m.showHelp = true; return nil }},
	}

	return map[string][]menuEntry{
		"file":     fileMenu,
		"view":     viewMenu,
		"navigate": navMenu,
	}
}

// handleMenuKey handles keyboard input when the menu overlay is open.
func (m *model) handleMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	menus := buildMenuEntries(m)
	entries := menus[m.activeMenuID]

	switch {
	case key.Matches(msg, m.keys.MenuClose), key.Matches(msg, m.keys.Help):
		m.menuOpen = false

	case key.Matches(msg, m.keys.MenuLeft):
		// Move to the previous menu category.
		for i, id := range menuIDs {
			if id == m.activeMenuID && i > 0 {
				m.activeMenuID = menuIDs[i-1]
				m.menuItemIndex = 0
				break
			}
		}

	case key.Matches(msg, m.keys.MenuRight):
		// Move to the next menu category.
		for i, id := range menuIDs {
			if id == m.activeMenuID && i < len(menuIDs)-1 {
				m.activeMenuID = menuIDs[i+1]
				m.menuItemIndex = 0
				break
			}
		}

	case key.Matches(msg, m.keys.MenuUp):
		if m.menuItemIndex > 0 {
			m.menuItemIndex--
			// Skip separators.
			for m.menuItemIndex > 0 && entries[m.menuItemIndex].separator {
				m.menuItemIndex--
			}
		}

	case key.Matches(msg, m.keys.MenuDown):
		if m.menuItemIndex < len(entries)-1 {
			m.menuItemIndex++
			// Skip separators.
			for m.menuItemIndex < len(entries)-1 && entries[m.menuItemIndex].separator {
				m.menuItemIndex++
			}
		}

	case key.Matches(msg, m.keys.MenuConfirm):
		if m.menuItemIndex >= 0 && m.menuItemIndex < len(entries) {
			entry := entries[m.menuItemIndex]
			if !entry.separator && entry.action != nil {
				m.menuOpen = false
				return m, entry.action(m)
			}
		}

	case key.Matches(msg, m.keys.Quit):
		m.menuOpen = false
	}

	return m, nil
}
