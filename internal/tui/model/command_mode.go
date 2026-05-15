package model

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kpango/unk/internal/loader"
	"github.com/kpango/unk/internal/tui/keys"
	"github.com/kpango/unk/internal/tui/styles"
	"github.com/kpango/unk/internal/types"
)

// executeCommand parses and executes the command-mode input (without leading ":").
// Any non-empty notice is written to m.updateNotice for the status bar.
// Returns an optional tea.Cmd for asynchronous side-effects (e.g. quit, reload).
func (m *model) executeCommand(input string) tea.Cmd {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	parts := strings.SplitN(input, " ", 2)
	name := strings.ToLower(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	switch name {

	// --- exit ---
	case "q", "q!", "quit":
		return tea.Quit

	// --- help ---
	case "h", "help":
		m.showHelp = true

	// --- layout modes ---
	case "split", "sp":
		m.setLayoutMode(types.LayoutModeSplit)

	case "stack", "st":
		m.setLayoutMode(types.LayoutModeStack)

	case "auto":
		m.setLayoutMode(types.LayoutModeAuto)

	// --- toggles ---
	case "wrap":
		m.wrapLines = !m.wrapLines
		m.invalidateFrame()

	case "number", "numbers", "nu":
		m.showLineNumbers = !m.showLineNumbers
		m.invalidateFrame()

	case "sidebar", "sb":
		m.toggleSidebar()
		m.invalidateFrame()

	case "headers", "hd":
		m.showUnkHeaders = !m.showUnkHeaders
		m.invalidateFrame()
		m.scrollTop = m.computeUnkScrollOffset()

	case "notes":
		m.showAgentNotes = !m.showAgentNotes
		m.clearRenderCache()

	// --- theme ---
	case "theme", "t":
		if arg == "" {
			m.themeID = styles.NextTheme(m.themeID)
		} else {
			m.themeID = arg
		}
		m.clearRenderCache()

	// --- reload ---
	case "reload", "e":
		if loader.CanReloadInput(m.bootstrap.Input) {
			return m.handleWatchTick()
		}
		m.updateNotice = "nothing to reload"
		m.statusBarDirty = true

	// --- file filter ---
	case "filter", "f":
		m.filter.SetValue(arg)
		m.markSidebarDirty()
		m.diffPaneCache = ""
		m.bodyCache = ""
		m.scrollTop = 0
		newCount := len(m.visibleFiles())
		if m.selectedFileIndex >= newCount {
			m.selectedFileIndex = max(newCount-1, 0)
		}

	// --- content search / grep ---
	case "search", "grep", "g":
		if arg != m.grepQuery {
			m.grepQuery = arg
			m.grepMatchLines = nil
			m.clearRenderCache()
			if arg != "" {
				m.computeGrepMatches()
				if len(m.grepMatchLines) > 0 {
					m.grepMatchIdx = 0
					m.scrollTop = max(0, m.grepMatchLines[0]-2)
				}
			}
		}

	// --- file jump ---
	case "first":
		files := m.visibleFiles()
		if len(files) > 0 {
			m.selectedFileIndex = 0
			m.selectedUnkIndex = 0
			m.scrollTop = m.computeUnkScrollOffset()
			m.markSidebarDirty()
		}

	case "last":
		files := m.visibleFiles()
		if len(files) > 0 {
			m.selectedFileIndex = len(files) - 1
			m.selectedUnkIndex = 0
			m.scrollTop = m.computeUnkScrollOffset()
			m.markSidebarDirty()
		}

	// --- keymap ---
	case "keymap", "km":
		if arg == "" {
			// Open interactive keymap selection overlay.
			m.keymapListIdx = keys.KeymapStyleIndex(m.keymapStyle)
			m.showKeymapList = true
		} else {
			m.keymapStyle = keys.NormalizeKeymapStyle(arg)
			m.keys = keys.ApplyOverrides(keys.KeyMapForStyle(m.keymapStyle), m.bootstrap.KeyBindingOverrides)
		}

	default:
		// Numeric argument: ":N" jumps to 1-based file index N.
		if n, err := strconv.Atoi(name); err == nil {
			files := m.visibleFiles()
			idx := n - 1
			if idx >= 0 && idx < len(files) {
				m.selectedFileIndex = idx
				m.selectedUnkIndex = 0
				m.scrollTop = m.computeUnkScrollOffset()
				m.markSidebarDirty()
			} else {
				m.updateNotice = "file index out of range"
				m.statusBarDirty = true
			}
		} else {
			m.updateNotice = "unknown command: " + name
			m.statusBarDirty = true
		}
	}

	return nil
}
