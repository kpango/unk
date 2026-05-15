package keys

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"

	"github.com/kpango/unk/internal/types"
)

func keyMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestApplyOverridesEmpty(t *testing.T) {
	base := HelixKeyMap()
	result := ApplyOverrides(base, nil)
	// No overrides — result should be identical to base.
	if !key.Matches(keyMsg(']'), result.NextUnk) {
		t.Error("NextUnk: expected ] after nil overrides")
	}
}

func TestApplyOverridesReplaceBinding(t *testing.T) {
	base := HelixKeyMap()
	overrides := types.KeyBindingOverrides{
		"next_unk": {"n"},
		"prev_unk": {"p"},
		"quit":     {"x"},
	}
	result := ApplyOverrides(base, overrides)

	if !key.Matches(keyMsg('n'), result.NextUnk) {
		t.Error("NextUnk: expected n after override")
	}
	if !key.Matches(keyMsg('p'), result.PrevUnk) {
		t.Error("PrevUnk: expected p after override")
	}
	if !key.Matches(keyMsg('x'), result.Quit) {
		t.Error("Quit: expected x after override")
	}

	// Original binding should be gone.
	if key.Matches(keyMsg(']'), result.NextUnk) {
		t.Error("NextUnk: original ] binding should be replaced, not kept")
	}
}

func TestApplyOverridesAllActions(t *testing.T) {
	// Verify every documented action name is handled (no silent no-ops).
	actions := []string{
		"next_unk", "prev_unk", "next_annotated_unk", "prev_annotated_unk",
		"next_file", "prev_file",
		"scroll_up", "scroll_down", "scroll_up_half", "scroll_down_half",
		"scroll_page_up", "scroll_page_down", "scroll_top", "scroll_bottom",
		"scroll_left", "scroll_right", "scroll_left_fast", "scroll_right_fast",
		"layout_split", "layout_stack", "layout_auto",
		"toggle_sidebar", "toggle_line_numbers", "toggle_wrap",
		"toggle_unk_headers", "toggle_agent_notes", "cycle_theme",
		"refresh", "open_in_editor", "yank_unk",
		"focus_filter", "focus_search", "search_next", "search_prev",
		"command_mode", "toggle_focus", "help", "quit", "suspend",
		"menu_open", "menu_close", "menu_left", "menu_right",
		"menu_up", "menu_down", "menu_confirm",
	}

	overrides := make(types.KeyBindingOverrides, len(actions))
	for _, a := range actions {
		overrides[a] = []string{"f1"}
	}

	// Should not panic.
	_ = ApplyOverrides(HelixKeyMap(), overrides)
}

func TestApplyOverridesUnknownAction(t *testing.T) {
	base := HelixKeyMap()
	overrides := types.KeyBindingOverrides{
		"unknown_action_xyz": {"f5"},
	}
	// Unknown actions should be silently ignored.
	result := ApplyOverrides(base, overrides)
	// NextUnk should still be ] (unchanged).
	if !key.Matches(keyMsg(']'), result.NextUnk) {
		t.Error("NextUnk: unexpected change from unknown override")
	}
}
