package config

import (
	"os"

	"gopkg.in/yaml.v3"

	"github.com/kpango/unk/internal/types"
)

// yamlCommandOptions holds display options for a per-command config section.
type yamlCommandOptions struct {
	Mode             string `yaml:"mode"`
	VCS              string `yaml:"vcs"`
	Theme            string `yaml:"theme"`
	Keymap           string `yaml:"keymap"`
	ExcludeUntracked *bool  `yaml:"exclude_untracked"`
	LineNumbers      *bool  `yaml:"line_numbers"`
	WrapLines        *bool  `yaml:"wrap_lines"`
	UnkHeaders       *bool  `yaml:"unk_headers"`
	AgentNotes       *bool  `yaml:"agent_notes"`
}

// yamlConfig is the shape of a unk config.yaml file on disk.
//
// Command-specific overrides live under `commands.<kind>`, which avoids the
// TOML key-collision between `vcs = "git"` (scalar) and `[vcs]` (section).
//
// Extended YAML-only fields:
//   - keybindings: per-action key overrides applied on top of the chosen keymap
//   - themes: user-defined color palettes that extend the built-in palette set
type yamlConfig struct {
	Mode             string `yaml:"mode"`
	VCS              string `yaml:"vcs"`
	Theme            string `yaml:"theme"`
	Keymap           string `yaml:"keymap"`
	ExcludeUntracked *bool  `yaml:"exclude_untracked"`
	LineNumbers      *bool  `yaml:"line_numbers"`
	WrapLines        *bool  `yaml:"wrap_lines"`
	UnkHeaders       *bool  `yaml:"unk_headers"`
	AgentNotes       *bool  `yaml:"agent_notes"`

	// Keybindings maps snake_case action names to key sequences.
	// Example:
	//   keybindings:
	//     next_unk: ["]"]
	//     scroll_down: ["j", "down"]
	Keybindings types.KeyBindingOverrides `yaml:"keybindings"`

	// Themes defines custom color palettes by name.
	// Example:
	//   themes:
	//     my-dark:
	//       panel: "#171a1d"
	//       accent: "#d5e0ea"
	//       ...
	Themes map[string]types.PaletteConfig `yaml:"themes"`

	// Commands holds per-command-kind settings (keyed by CLIInput.Kind()).
	// Example:
	//   commands:
	//     vcs:
	//       mode: split
	//     show:
	//       theme: paper
	Commands map[string]*yamlCommandOptions `yaml:"commands"`
}

// readYAMLConfig parses one config.yaml (or config.yml) file.
// Silently returns nil for missing files.
func readYAMLConfig(path string) (*yamlConfig, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg yamlConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// resolveYAMLConfigPath returns the first YAML config file path that exists
// under the given base directory (prefers .yaml over .yml).
// Returns "" when neither exists.
func resolveYAMLConfigPath(dir string) string {
	for _, name := range []string{"config.yaml", "config.yml"} {
		p := dir + "/" + name
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// yamlToOptions converts a parsed YAML config (root or command section) into
// a CommonOptions overlay.
func yamlToOptions(cfg *yamlCommandOptions) types.CommonOptions {
	var opts types.CommonOptions
	if m := normalizeLayoutMode(cfg.Mode); m != "" {
		opts.Mode = types.Ptr(m)
	}
	if v := normalizeVCSMode(cfg.VCS); v != "" {
		opts.VCS = types.Ptr(v)
	}
	if cfg.Theme != "" {
		opts.Theme = types.Ptr(cfg.Theme)
	}
	if k := normalizeKeymapStyle(cfg.Keymap); k != "" {
		opts.Keymap = types.Ptr(k)
	}
	opts.ExcludeUntracked = cfg.ExcludeUntracked
	opts.LineNumbers = cfg.LineNumbers
	opts.WrapLines = cfg.WrapLines
	opts.UnkHeaders = cfg.UnkHeaders
	opts.AgentNotes = cfg.AgentNotes
	return opts
}

// yamlCommonOptions converts a full yamlConfig (with command-section resolution)
// into a CommonOptions overlay.
func yamlCommonOptions(cfg *yamlConfig, kind string, pagerMode bool) types.CommonOptions {
	root := &yamlCommandOptions{
		Mode:             cfg.Mode,
		VCS:              cfg.VCS,
		Theme:            cfg.Theme,
		Keymap:           cfg.Keymap,
		ExcludeUntracked: cfg.ExcludeUntracked,
		LineNumbers:      cfg.LineNumbers,
		WrapLines:        cfg.WrapLines,
		UnkHeaders:       cfg.UnkHeaders,
		AgentNotes:       cfg.AgentNotes,
	}
	base := yamlToOptions(root)

	if sec, ok := cfg.Commands[kind]; ok && sec != nil {
		base = mergeOptions(base, yamlToOptions(sec))
	}

	if pagerMode {
		if sec, ok := cfg.Commands["pager"]; ok && sec != nil {
			base = mergeOptions(base, yamlToOptions(sec))
		}
	}

	return base
}

// mergeKeyBindingOverrides merges src into dst, with src taking precedence for
// duplicate action names. dst is modified in place and returned.
func mergeKeyBindingOverrides(dst, src types.KeyBindingOverrides) types.KeyBindingOverrides {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(types.KeyBindingOverrides, len(src))
	}
	for action, keys := range src {
		dst[action] = keys
	}
	return dst
}

// mergeCustomPalettes merges src into dst, with src taking precedence for
// duplicate palette names. dst is modified in place and returned.
func mergeCustomPalettes(dst, src map[string]types.PaletteConfig) map[string]types.PaletteConfig {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]types.PaletteConfig, len(src))
	}
	for name, p := range src {
		dst[name] = p
	}
	return dst
}
