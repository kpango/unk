package keys

import (
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/key"

	"github.com/kpango/unk/internal/types"
)

// KeymapStyles is the ordered list of supported keymap style names.
var KeymapStyles = []string{"helix", "vim", "emacs"}

// KeymapStyleIndex returns the index of style in KeymapStyles, defaulting to 0.
func KeymapStyleIndex(style string) int {
	for i, s := range KeymapStyles {
		if s == style {
			return i
		}
	}
	return 0
}

// NormalizeKeymapStyle canonicalizes a keymap name to "helix", "vim", or "emacs".
func NormalizeKeymapStyle(s string) string {
	switch strings.ToLower(s) {
	case "vim":
		return "vim"
	case "emacs":
		return "emacs"
	default:
		return "helix"
	}
}

// CommandCompletions returns all command names that begin with prefix (sorted).
func CommandCompletions(prefix string) []string {
	all := []string{
		"auto", "e", "f", "filter", "first", "g", "grep",
		"h", "hd", "headers", "help",
		"keymap", "km",
		"last",
		"notes", "nu", "number", "numbers",
		"q", "q!", "quit",
		"reload",
		"sb", "search", "sidebar", "sp", "split", "st", "stack",
		"t", "theme",
		"wrap",
	}
	if prefix == "" {
		return all
	}
	prefix = strings.ToLower(prefix)
	var out []string
	for _, c := range all {
		if strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}

// keyMapOnce ensures each keymap is built exactly once. The built maps are
// returned by value (struct copy) on every subsequent call — copying slice
// headers is zero-alloc and safe because Binding fields are never mutated
// after construction.
var (
	helixOnce  sync.Once
	helixBuilt KeyMap
	vimOnce    sync.Once
	vimBuilt   KeyMap
	emacsOnce  sync.Once
	emacsBuilt KeyMap
)

// KeyMap holds all keyboard bindings for the unk TUI.
type KeyMap struct {
	// Navigation
	PrevUnk          key.Binding
	NextUnk          key.Binding
	PrevAnnotatedUnk key.Binding
	NextAnnotatedUnk key.Binding
	PrevFile         key.Binding
	NextFile         key.Binding

	// Scrolling
	ScrollUp        key.Binding
	ScrollDown      key.Binding
	ScrollUpHalf    key.Binding
	ScrollDownHalf  key.Binding
	ScrollPageUp    key.Binding
	ScrollPageDown  key.Binding
	ScrollTop       key.Binding
	ScrollBottom    key.Binding
	ScrollLeft      key.Binding
	ScrollRight     key.Binding
	ScrollLeftFast  key.Binding
	ScrollRightFast key.Binding

	// Layout
	LayoutSplit key.Binding
	LayoutStack key.Binding
	LayoutAuto  key.Binding

	// Toggles
	ToggleSidebar     key.Binding
	ToggleLineNumbers key.Binding
	ToggleWrap        key.Binding
	ToggleUnkHeaders  key.Binding
	ToggleAgentNotes  key.Binding
	CycleTheme        key.Binding

	// App actions
	Refresh      key.Binding
	OpenInEditor key.Binding
	YankUnk      key.Binding
	FocusFilter  key.Binding
	FocusSearch  key.Binding
	SearchNext   key.Binding
	SearchPrev   key.Binding
	CommandMode  key.Binding
	ToggleFocus  key.Binding
	Help         key.Binding
	Quit         key.Binding
	Suspend      key.Binding

	// Menu
	MenuOpen    key.Binding
	MenuClose   key.Binding
	MenuLeft    key.Binding
	MenuRight   key.Binding
	MenuUp      key.Binding
	MenuDown    key.Binding
	MenuConfirm key.Binding
}

// ApplyOverrides returns a copy of km with the specified bindings replaced.
// The overrides map uses snake_case action names to lists of key sequences.
//
// Supported action names:
//
//	next_unk, prev_unk, next_annotated_unk, prev_annotated_unk,
//	next_file, prev_file,
//	scroll_up, scroll_down, scroll_up_half, scroll_down_half,
//	scroll_page_up, scroll_page_down, scroll_top, scroll_bottom,
//	scroll_left, scroll_right, scroll_left_fast, scroll_right_fast,
//	layout_split, layout_stack, layout_auto,
//	toggle_sidebar, toggle_line_numbers, toggle_wrap, toggle_unk_headers,
//	toggle_agent_notes, cycle_theme,
//	refresh, open_in_editor, yank_unk,
//	focus_filter, focus_search, search_next, search_prev,
//	command_mode, toggle_focus, help, quit, suspend,
//	menu_open, menu_close, menu_left, menu_right, menu_up, menu_down, menu_confirm
func ApplyOverrides(km KeyMap, overrides types.KeyBindingOverrides) KeyMap {
	if len(overrides) == 0 {
		return km
	}
	for action, ks := range overrides {
		if len(ks) == 0 {
			continue
		}
		b := key.NewBinding(key.WithKeys(ks...))
		switch action {
		case "next_unk":
			km.NextUnk = b
		case "prev_unk":
			km.PrevUnk = b
		case "next_annotated_unk":
			km.NextAnnotatedUnk = b
		case "prev_annotated_unk":
			km.PrevAnnotatedUnk = b
		case "next_file":
			km.NextFile = b
		case "prev_file":
			km.PrevFile = b
		case "scroll_up":
			km.ScrollUp = b
		case "scroll_down":
			km.ScrollDown = b
		case "scroll_up_half":
			km.ScrollUpHalf = b
		case "scroll_down_half":
			km.ScrollDownHalf = b
		case "scroll_page_up":
			km.ScrollPageUp = b
		case "scroll_page_down":
			km.ScrollPageDown = b
		case "scroll_top":
			km.ScrollTop = b
		case "scroll_bottom":
			km.ScrollBottom = b
		case "scroll_left":
			km.ScrollLeft = b
		case "scroll_right":
			km.ScrollRight = b
		case "scroll_left_fast":
			km.ScrollLeftFast = b
		case "scroll_right_fast":
			km.ScrollRightFast = b
		case "layout_split":
			km.LayoutSplit = b
		case "layout_stack":
			km.LayoutStack = b
		case "layout_auto":
			km.LayoutAuto = b
		case "toggle_sidebar":
			km.ToggleSidebar = b
		case "toggle_line_numbers":
			km.ToggleLineNumbers = b
		case "toggle_wrap":
			km.ToggleWrap = b
		case "toggle_unk_headers":
			km.ToggleUnkHeaders = b
		case "toggle_agent_notes":
			km.ToggleAgentNotes = b
		case "cycle_theme":
			km.CycleTheme = b
		case "refresh":
			km.Refresh = b
		case "open_in_editor":
			km.OpenInEditor = b
		case "yank_unk":
			km.YankUnk = b
		case "focus_filter":
			km.FocusFilter = b
		case "focus_search":
			km.FocusSearch = b
		case "search_next":
			km.SearchNext = b
		case "search_prev":
			km.SearchPrev = b
		case "command_mode":
			km.CommandMode = b
		case "toggle_focus":
			km.ToggleFocus = b
		case "help":
			km.Help = b
		case "quit":
			km.Quit = b
		case "suspend":
			km.Suspend = b
		case "menu_open":
			km.MenuOpen = b
		case "menu_close":
			km.MenuClose = b
		case "menu_left":
			km.MenuLeft = b
		case "menu_right":
			km.MenuRight = b
		case "menu_up":
			km.MenuUp = b
		case "menu_down":
			km.MenuDown = b
		case "menu_confirm":
			km.MenuConfirm = b
		}
	}
	return km
}

// DefaultKeyMap returns the default key bindings (Helix style).
func DefaultKeyMap() KeyMap { return HelixKeyMap() }

// KeyMapForStyle returns key bindings for the named style.
// Recognized styles: "helix" (default), "vim", "emacs".
// Unknown or empty strings fall back to Helix.
func KeyMapForStyle(style string) KeyMap {
	switch style {
	case "vim":
		return VimKeyMap()
	case "emacs":
		return EmacsKeyMap()
	default:
		return HelixKeyMap()
	}
}

// HelixKeyMap returns Helix-Editor-style key bindings (cached via sync.Once).
func HelixKeyMap() KeyMap {
	helixOnce.Do(func() { helixBuilt = BuildHelixKeyMap() })
	return helixBuilt
}

// VimKeyMap returns Vim-style key bindings (cached via sync.Once).
func VimKeyMap() KeyMap {
	vimOnce.Do(func() { vimBuilt = BuildVimKeyMap() })
	return vimBuilt
}

// EmacsKeyMap returns Emacs-style key bindings (cached via sync.Once).
func EmacsKeyMap() KeyMap {
	emacsOnce.Do(func() { emacsBuilt = BuildEmacsKeyMap() })
	return emacsBuilt
}
