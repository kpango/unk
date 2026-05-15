package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kpango/unk/internal/types"
)

func TestYAMLConfigBasicOptions(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	content := `
mode: split
vcs: git
theme: midnight
keymap: vim
line_numbers: false
wrap_lines: true
unk_headers: false
agent_notes: true
exclude_untracked: true
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := readYAMLConfig(cfgPath)
	if err != nil {
		t.Fatalf("readYAMLConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Mode != "split" {
		t.Errorf("mode: want split, got %q", cfg.Mode)
	}
	if cfg.Theme != "midnight" {
		t.Errorf("theme: want midnight, got %q", cfg.Theme)
	}
	if cfg.Keymap != "vim" {
		t.Errorf("keymap: want vim, got %q", cfg.Keymap)
	}
	if cfg.LineNumbers == nil || *cfg.LineNumbers {
		t.Errorf("line_numbers: want false, got %v", cfg.LineNumbers)
	}
	if cfg.WrapLines == nil || !*cfg.WrapLines {
		t.Errorf("wrap_lines: want true, got %v", cfg.WrapLines)
	}
}

func TestYAMLConfigMissingFile(t *testing.T) {
	cfg, err := readYAMLConfig("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config for missing file, got non-nil")
	}
}

func TestYAMLConfigKeybindingOverrides(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	content := `
keybindings:
  next_unk: ["]", "ctrl+n"]
  prev_unk: ["[", "ctrl+p"]
  quit: ["q"]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := readYAMLConfig(cfgPath)
	if err != nil {
		t.Fatalf("readYAMLConfig: %v", err)
	}
	if len(cfg.Keybindings) != 3 {
		t.Errorf("keybindings: want 3 entries, got %d", len(cfg.Keybindings))
	}
	if got := cfg.Keybindings["next_unk"]; len(got) != 2 || got[0] != "]" || got[1] != "ctrl+n" {
		t.Errorf("next_unk: want [\"]\", \"ctrl+n\"], got %v", got)
	}
}

func TestYAMLConfigCustomThemes(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	content := `
themes:
  my-dark:
    panel_alt: "#1d2126"
    panel: "#171a1d"
    border: "#343c45"
    accent: "#d5e0ea"
    text: "#f2f4f6"
    muted: "#9aa4af"
    accent_muted: "#414a54"
    added_bg: "#1f3025"
    removed_bg: "#372526"
    context_bg: "#181c20"
    added_content_bg: "#24362a"
    removed_content_bg: "#432b2d"
    added_sign_color: "#88d39b"
    removed_sign_color: "#f0a0a0"
    unk_header_fg: "#9aa4af"
    line_number_bg: "#14181b"
    line_number_fg: "#798592"
    badge_added: "#88d39b"
    badge_removed: "#f0a0a0"
    badge_neutral: "#a9b4bf"
    file_new: "#88d39b"
    file_deleted: "#f0a0a0"
    file_renamed: "#e6cf98"
    file_modified: "#c49bff"
    file_untracked: "#7fd1ff"
    note_border: "#c6a0ff"
    note_background: "#241c31"
    note_title_background: "#322446"
    note_title_text: "#f5edff"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := readYAMLConfig(cfgPath)
	if err != nil {
		t.Fatalf("readYAMLConfig: %v", err)
	}
	if len(cfg.Themes) != 1 {
		t.Errorf("themes: want 1 entry, got %d", len(cfg.Themes))
	}
	p, ok := cfg.Themes["my-dark"]
	if !ok {
		t.Fatal("themes: missing my-dark")
	}
	if p.Panel != "#171a1d" {
		t.Errorf("my-dark panel: want #171a1d, got %q", p.Panel)
	}
	if p.Accent != "#d5e0ea" {
		t.Errorf("my-dark accent: want #d5e0ea, got %q", p.Accent)
	}
}

func TestYAMLConfigCommandSections(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	content := `
theme: graphite
commands:
  vcs:
    mode: split
  show:
    theme: paper
    line_numbers: false
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := readYAMLConfig(cfgPath)
	if err != nil {
		t.Fatalf("readYAMLConfig: %v", err)
	}
	if cfg.Theme != "graphite" {
		t.Errorf("root theme: want graphite, got %q", cfg.Theme)
	}
	vcsSec, ok := cfg.Commands["vcs"]
	if !ok || vcsSec == nil {
		t.Fatal("commands.vcs: missing section")
	}
	if vcsSec.Mode != "split" {
		t.Errorf("commands.vcs.mode: want split, got %q", vcsSec.Mode)
	}
	showSec, ok := cfg.Commands["show"]
	if !ok || showSec == nil {
		t.Fatal("commands.show: missing section")
	}
	if showSec.Theme != "paper" {
		t.Errorf("commands.show.theme: want paper, got %q", showSec.Theme)
	}
	if showSec.LineNumbers == nil || *showSec.LineNumbers {
		t.Errorf("commands.show.line_numbers: want false, got %v", showSec.LineNumbers)
	}
}

func TestYAMLResolveIntegration(t *testing.T) {
	tmp := t.TempDir()

	// Plant repo root
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Write YAML repo config
	unkDir := filepath.Join(tmp, ".unk")
	if err := os.MkdirAll(unkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `
mode: split
theme: midnight
keymap: vim
keybindings:
  quit: ["x", "q"]
themes:
  custom-blue:
    panel: "#001122"
    accent: "#0088ff"
`
	if err := os.WriteFile(filepath.Join(unkDir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	input := &types.VCSInput{}
	result, err := Resolve(input, tmp)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	opts := types.OptionsOf(result.Input)
	if opts.Mode == nil || *opts.Mode != types.LayoutModeSplit {
		t.Errorf("mode: want split, got %v", opts.Mode)
	}
	if opts.Theme == nil || *opts.Theme != "midnight" {
		t.Errorf("theme: want midnight, got %v", opts.Theme)
	}
	if opts.Keymap == nil || *opts.Keymap != "vim" {
		t.Errorf("keymap: want vim, got %v", opts.Keymap)
	}

	// Keybinding overrides should be propagated
	if len(result.KeyBindingOverrides) == 0 {
		t.Error("KeyBindingOverrides: want non-empty")
	}
	if got := result.KeyBindingOverrides["quit"]; len(got) != 2 || got[0] != "x" {
		t.Errorf("quit override: want [x, q], got %v", got)
	}

	// Custom palette should be propagated
	if len(result.CustomPalettes) == 0 {
		t.Error("CustomPalettes: want non-empty")
	}
	if p, ok := result.CustomPalettes["custom-blue"]; !ok {
		t.Error("CustomPalettes: missing custom-blue")
	} else if p.Panel != "#001122" {
		t.Errorf("custom-blue.panel: want #001122, got %q", p.Panel)
	}
}

func TestEmbeddedDefaultsApplied(t *testing.T) {
	// With no config files and no CLI flags, the embedded defaults.yaml
	// must produce the documented default values.
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	result, err := Resolve(&types.VCSInput{}, tmp)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	opts := types.OptionsOf(result.Input)

	if opts.Mode == nil || *opts.Mode != types.LayoutModeAuto {
		t.Errorf("mode: want auto, got %v", opts.Mode)
	}
	if opts.Theme == nil || *opts.Theme != "graphite" {
		t.Errorf("theme: want graphite, got %v", opts.Theme)
	}
	if opts.Keymap == nil || *opts.Keymap != "helix" {
		t.Errorf("keymap: want helix, got %v", opts.Keymap)
	}
	if opts.LineNumbers == nil || !*opts.LineNumbers {
		t.Errorf("line_numbers: want true, got %v", opts.LineNumbers)
	}
	if opts.WrapLines == nil || *opts.WrapLines {
		t.Errorf("wrap_lines: want false, got %v", opts.WrapLines)
	}
	if opts.UnkHeaders == nil || !*opts.UnkHeaders {
		t.Errorf("unk_headers: want true, got %v", opts.UnkHeaders)
	}
	if opts.AgentNotes == nil || *opts.AgentNotes {
		t.Errorf("agent_notes: want false, got %v", opts.AgentNotes)
	}
	if opts.ExcludeUntracked == nil || *opts.ExcludeUntracked {
		t.Errorf("exclude_untracked: want false, got %v", opts.ExcludeUntracked)
	}
}

func TestUserConfigOverridesDefaults(t *testing.T) {
	// A repo config.yaml that sets only theme and wrap_lines should override
	// those two values but leave all other defaults intact.
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	unkDir := filepath.Join(tmp, ".unk")
	if err := os.MkdirAll(unkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "theme: midnight\nwrap_lines: true\n"
	if err := os.WriteFile(filepath.Join(unkDir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	result, err := Resolve(&types.VCSInput{}, tmp)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	opts := types.OptionsOf(result.Input)

	// Overridden values
	if opts.Theme == nil || *opts.Theme != "midnight" {
		t.Errorf("theme: want midnight, got %v", opts.Theme)
	}
	if opts.WrapLines == nil || !*opts.WrapLines {
		t.Errorf("wrap_lines: want true (overridden), got %v", opts.WrapLines)
	}

	// Unset values should still come from embedded defaults
	if opts.Mode == nil || *opts.Mode != types.LayoutModeAuto {
		t.Errorf("mode: want auto (from defaults), got %v", opts.Mode)
	}
	if opts.Keymap == nil || *opts.Keymap != "helix" {
		t.Errorf("keymap: want helix (from defaults), got %v", opts.Keymap)
	}
	if opts.LineNumbers == nil || !*opts.LineNumbers {
		t.Errorf("line_numbers: want true (from defaults), got %v", opts.LineNumbers)
	}
}

func TestMergeKeyBindingOverrides(t *testing.T) {
	base := types.KeyBindingOverrides{"quit": {"q"}, "help": {"?"}}
	override := types.KeyBindingOverrides{"quit": {"x", "q"}, "refresh": {"r"}}

	result := mergeKeyBindingOverrides(base, override)
	if len(result) != 3 {
		t.Errorf("want 3 entries, got %d", len(result))
	}
	if got := result["quit"]; len(got) != 2 || got[0] != "x" {
		t.Errorf("quit: want [x, q], got %v", got)
	}
	if got := result["help"]; len(got) != 1 || got[0] != "?" {
		t.Errorf("help: want [?], got %v", got)
	}
	if got := result["refresh"]; len(got) != 1 || got[0] != "r" {
		t.Errorf("refresh: want [r], got %v", got)
	}
}

func TestMergeCustomPalettes(t *testing.T) {
	base := map[string]types.PaletteConfig{
		"a": {Panel: "#aaa"},
	}
	override := map[string]types.PaletteConfig{
		"a": {Panel: "#bbb"}, // overrides
		"b": {Panel: "#ccc"}, // new
	}

	result := mergeCustomPalettes(base, override)
	if len(result) != 2 {
		t.Errorf("want 2 entries, got %d", len(result))
	}
	if result["a"].Panel != "#bbb" {
		t.Errorf("palette a.panel: want #bbb, got %q", result["a"].Panel)
	}
	if result["b"].Panel != "#ccc" {
		t.Errorf("palette b.panel: want #ccc, got %q", result["b"].Panel)
	}
}

func TestYAMLYMLFallback(t *testing.T) {
	// resolveYAMLConfigPath should find .yml when .yaml doesn't exist
	tmp := t.TempDir()
	ymlPath := filepath.Join(tmp, "config.yml")
	if err := os.WriteFile(ymlPath, []byte("mode: stack\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := resolveYAMLConfigPath(tmp)
	if got != ymlPath {
		t.Errorf("want %q, got %q", ymlPath, got)
	}
}
