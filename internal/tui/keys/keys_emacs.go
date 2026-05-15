package keys

import "github.com/charmbracelet/bubbles/key"

// BuildEmacsKeyMap builds Emacs-style key bindings without caching.
func BuildEmacsKeyMap() KeyMap {
	return KeyMap{
		PrevUnk:          key.NewBinding(key.WithKeys("["), key.WithHelp("[", "prev unk")),
		NextUnk:          key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next unk")),
		PrevAnnotatedUnk: key.NewBinding(key.WithKeys("{"), key.WithHelp("{", "prev ann. unk")),
		NextAnnotatedUnk: key.NewBinding(key.WithKeys("}"), key.WithHelp("}", "next ann. unk")),
		PrevFile:         key.NewBinding(key.WithKeys("("), key.WithHelp("(", "prev file")),
		NextFile:         key.NewBinding(key.WithKeys(")"), key.WithHelp(")", "next file")),

		ScrollUp:        key.NewBinding(key.WithKeys("up", "ctrl+p"), key.WithHelp("↑/C-p", "scroll up")),
		ScrollDown:      key.NewBinding(key.WithKeys("down", "ctrl+n"), key.WithHelp("↓/C-n", "scroll down")),
		ScrollUpHalf:    key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("C-u", "half page up")),
		ScrollDownHalf:  key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("C-d", "half page down")),
		ScrollPageUp:    key.NewBinding(key.WithKeys("pgup", "alt+v"), key.WithHelp("M-v/pgup", "page up")),
		ScrollPageDown:  key.NewBinding(key.WithKeys("pgdown", "ctrl+v", " "), key.WithHelp("C-v/pgdn", "page down")),
		ScrollTop:       key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "top")),
		ScrollBottom:    key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "bottom")),
		ScrollLeft:      key.NewBinding(key.WithKeys("left", "ctrl+b"), key.WithHelp("←/C-b", "scroll left")),
		ScrollRight:     key.NewBinding(key.WithKeys("right", "ctrl+f"), key.WithHelp("→/C-f", "scroll right")),
		ScrollLeftFast:  key.NewBinding(key.WithKeys("shift+left"), key.WithHelp("S-←", "scroll left ×8")),
		ScrollRightFast: key.NewBinding(key.WithKeys("shift+right"), key.WithHelp("S-→", "scroll right ×8")),

		LayoutSplit: key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "split layout")),
		LayoutStack: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "stack layout")),
		LayoutAuto:  key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "auto layout")),

		ToggleSidebar:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "toggle sidebar")),
		ToggleLineNumbers: key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "line numbers")),
		ToggleWrap:        key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "wrap lines")),
		ToggleUnkHeaders:  key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "unk headers")),
		ToggleAgentNotes:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "agent notes")),
		CycleTheme:        key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "cycle theme")),

		Refresh:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		OpenInEditor: key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open in editor")),
		YankUnk:      key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yank unk")),
		FocusFilter:  key.NewBinding(key.WithKeys("ctrl+s", "/"), key.WithHelp("C-s", "filter files")),
		FocusSearch:  key.NewBinding(key.WithKeys("\\"), key.WithHelp("\\", "grep content")),
		SearchNext:   key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next match")),
		SearchPrev:   key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "prev match")),
		CommandMode:  key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command")),
		ToggleFocus:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "focus")),
		Help:         key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:         key.NewBinding(key.WithKeys("q", "esc", "ctrl+c", "ctrl+g"), key.WithHelp("q/C-g", "quit")),
		Suspend:      key.NewBinding(key.WithKeys("ctrl+z")),

		MenuOpen:    key.NewBinding(key.WithKeys("f10")),
		MenuClose:   key.NewBinding(key.WithKeys("esc", "ctrl+g")),
		MenuLeft:    key.NewBinding(key.WithKeys("left")),
		MenuRight:   key.NewBinding(key.WithKeys("right", "tab")),
		MenuUp:      key.NewBinding(key.WithKeys("up")),
		MenuDown:    key.NewBinding(key.WithKeys("down")),
		MenuConfirm: key.NewBinding(key.WithKeys("enter")),
	}
}
