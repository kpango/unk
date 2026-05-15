// Package config resolves the layered configuration for a unk invocation.
package config

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"

	"github.com/kpango/unk/internal/types"
)

//go:embed defaults.yaml
var defaultsYAML []byte

// parseEmbeddedDefaults parses the embedded defaults.yaml into CommonOptions.
// Panics on parse error — a malformed embedded file is a programming error.
func parseEmbeddedDefaults() types.CommonOptions {
	var cfg yamlConfig
	if err := yaml.Unmarshal(defaultsYAML, &cfg); err != nil {
		panic("unk: failed to parse embedded defaults.yaml: " + err.Error())
	}
	return yamlToOptions(&yamlCommandOptions{
		Mode:             cfg.Mode,
		Theme:            cfg.Theme,
		Keymap:           cfg.Keymap,
		ExcludeUntracked: cfg.ExcludeUntracked,
		LineNumbers:      cfg.LineNumbers,
		WrapLines:        cfg.WrapLines,
		UnkHeaders:       cfg.UnkHeaders,
		AgentNotes:       cfg.AgentNotes,
	})
}

// tomlConfig is the shape of a unk config.toml file on disk.
// TOML keys use snake_case; we decode directly with struct tags.
//
// NOTE: the "vcs" key is used for BOTH the scalar `vcs = "git"` (VCS backend)
// and the `[vcs]` table section (unk diff command settings). BurntSushi silently
// drops both when the same tag appears on two fields. We work around this by
// NOT putting a toml tag on VCS here and populating it from a separate decode pass
// (see readTomlConfig).
type tomlConfig struct {
	Mode             string `toml:"mode"`
	VCS              string // populated by readTomlConfig's second decode pass
	Theme            string `toml:"theme"`
	Keymap           string `toml:"keymap"`
	ExcludeUntracked *bool  `toml:"exclude_untracked"`
	LineNumbers      *bool  `toml:"line_numbers"`
	WrapLines        *bool  `toml:"wrap_lines"`
	UnkHeaders       *bool  `toml:"unk_headers"`
	AgentNotes       *bool  `toml:"agent_notes"`

	// Themes defines custom color palettes by name.
	// Example in TOML:
	//   [themes.my-dark]
	//   panel = "#171a1d"
	//   accent = "#d5e0ea"
	Themes map[string]types.PaletteConfig `toml:"themes"`

	// Command sections keyed by CLIInput.Kind().
	VcsSection      *tomlConfig `toml:"vcs"`
	ShowSection     *tomlConfig `toml:"show"`
	StashSection    *tomlConfig `toml:"stash-show"`
	DiffSection     *tomlConfig `toml:"diff"`
	PatchSection    *tomlConfig `toml:"patch"`
	DifftoolSection *tomlConfig `toml:"difftool"`
	PagerSection    *tomlConfig `toml:"pager"`
}

// tomlVCSScalar is used to decode just the `vcs = "..."` scalar to avoid the
// struct-tag collision between the VCS string and the [vcs] section.
type tomlVCSScalar struct {
	VCS string `toml:"vcs"`
}

// Resolution is the fully-resolved config for one invocation.
type Resolution struct {
	Input          types.CLIInput
	GlobalCfgPath  *string
	RepoCfgPath    *string
	// KeyBindingOverrides holds merged per-action key overrides from all config
	// layers. Applied on top of the base keymap after resolution.
	KeyBindingOverrides types.KeyBindingOverrides
	// CustomPalettes holds merged user-defined color palettes from all config layers.
	CustomPalettes map[string]types.PaletteConfig
}

// Resolve applies the full config precedence chain to input:
//
//	built-ins → global TOML → global YAML → repo TOML → repo YAML → CLI flags
//
// Keybinding overrides and custom palettes are accumulated across all layers
// (each layer adds/replaces individual keys rather than wholesale replacing the
// previous layer's map).
func Resolve(input types.CLIInput, cwd string) (*Resolution, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	repoRoot := FindRepoRoot(cwd)
	vcsMode := DetectVCSMode(repoRoot)

	// Start from the embedded defaults (defaults.yaml), then layer overrides on top.
	base := parseEmbeddedDefaults()
	// VCS backend is auto-detected, not part of the static defaults.
	base.VCS = types.Ptr(vcsMode)
	// pager and watch are CLI-only flags; always start false.
	base.Pager = types.Ptr(false)
	base.Watch = types.Ptr(false)

	// Preserve CLI-supplied agent-context and pager/watch flags upfront.
	opts := types.OptionsOf(input)
	base.AgentContext = opts.AgentContext

	pagerMode := opts.Pager != nil && *opts.Pager

	var (
		globalCfgPath       *string
		keyBindingOverrides types.KeyBindingOverrides
		customPalettes      map[string]types.PaletteConfig
	)

	// ── Global config ─────────────────────────────────────────────────────────
	if globalDir := resolveGlobalConfigDir(); globalDir != "" {
		// TOML
		tomlPath := filepath.Join(globalDir, "config.toml")
		globalCfgPath = &tomlPath
		if raw, err := readTomlConfig(tomlPath); err == nil && raw != nil {
			base = mergeOptions(base, tomlCommonOptions(raw, input.Kind(), pagerMode))
			customPalettes = mergeCustomPalettes(customPalettes, raw.Themes)
		}
		// YAML (takes precedence over TOML at the same level)
		if yamlPath := resolveYAMLConfigPath(globalDir); yamlPath != "" {
			globalCfgPath = &yamlPath
			if raw, err := readYAMLConfig(yamlPath); err == nil && raw != nil {
				base = mergeOptions(base, yamlCommonOptions(raw, input.Kind(), pagerMode))
				keyBindingOverrides = mergeKeyBindingOverrides(keyBindingOverrides, raw.Keybindings)
				customPalettes = mergeCustomPalettes(customPalettes, raw.Themes)
			}
		}
	}

	// ── Repo-level config ──────────────────────────────────────────────────────
	var repoCfgPath *string
	if repoRoot != "" {
		unkDir := filepath.Join(repoRoot, ".unk")
		// TOML
		tomlPath := filepath.Join(unkDir, "config.toml")
		repoCfgPath = &tomlPath
		if raw, err := readTomlConfig(tomlPath); err == nil && raw != nil {
			base = mergeOptions(base, tomlCommonOptions(raw, input.Kind(), pagerMode))
			customPalettes = mergeCustomPalettes(customPalettes, raw.Themes)
		}
		// YAML (takes precedence over TOML at the same level)
		if yamlPath := resolveYAMLConfigPath(unkDir); yamlPath != "" {
			repoCfgPath = &yamlPath
			if raw, err := readYAMLConfig(yamlPath); err == nil && raw != nil {
				base = mergeOptions(base, yamlCommonOptions(raw, input.Kind(), pagerMode))
				keyBindingOverrides = mergeKeyBindingOverrides(keyBindingOverrides, raw.Keybindings)
				customPalettes = mergeCustomPalettes(customPalettes, raw.Themes)
			}
		}
	}

	// ── CLI flags win over everything ─────────────────────────────────────────
	base = mergeOptions(base, opts)
	// Let CLI pager/watch/agent flags take final precedence over config files.
	base.Pager = types.Ptr(opts.Pager != nil && *opts.Pager)
	base.Watch = types.Ptr(opts.Watch != nil && *opts.Watch)
	base.AgentContext = opts.AgentContext

	resolved := withOptions(input, base)
	return &Resolution{
		Input:               resolved,
		GlobalCfgPath:       globalCfgPath,
		RepoCfgPath:         repoCfgPath,
		KeyBindingOverrides: keyBindingOverrides,
		CustomPalettes:      customPalettes,
	}, nil
}

// resolveGlobalConfigDir returns the platform-appropriate unk config directory.
// Precedence: $XDG_CONFIG_HOME > $HOME/.config > os.UserConfigDir() (covers %APPDATA% on Windows).
func resolveGlobalConfigDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		if home := os.Getenv("HOME"); home != "" {
			base = filepath.Join(home, ".config")
		} else if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		} else {
			return ""
		}
	}
	return filepath.Join(base, "unk")
}

// readTomlConfig parses one config.toml file; silently skips missing files.
// It performs two decode passes to handle the "vcs" key ambiguity: the key is
// used for both the `vcs = "git"` scalar (VCS backend) and the `[vcs]` section
// (unk diff command settings). A single struct can't hold both with the same tag.
func readTomlConfig(path string) (*tomlConfig, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	var cfg tomlConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}
	// Separately decode the vcs scalar to avoid the struct-tag collision.
	var scalars tomlVCSScalar
	toml.DecodeFile(path, &scalars) //nolint:errcheck — best-effort; mismatch is expected when [vcs] section is present
	cfg.VCS = scalars.VCS
	return &cfg, nil
}

// tomlCommonOptions converts one parsed TOML object (with command/pager section
// resolution) into a CommonOptions overlay.
func tomlCommonOptions(cfg *tomlConfig, kind string, pagerMode bool) types.CommonOptions {
	base := tomlToOptions(cfg)

	// Apply the command-specific section if present.
	if sec := commandSection(cfg, kind); sec != nil {
		base = mergeOptions(base, tomlToOptions(sec))
	}

	// Apply the [pager] section when pager mode is active.
	if pagerMode && cfg.PagerSection != nil {
		base = mergeOptions(base, tomlToOptions(cfg.PagerSection))
	}

	return base
}

func commandSection(cfg *tomlConfig, kind string) *tomlConfig {
	switch kind {
	case "vcs":
		return cfg.VcsSection
	case "show":
		return cfg.ShowSection
	case "stash-show":
		return cfg.StashSection
	case "diff":
		return cfg.DiffSection
	case "patch":
		return cfg.PatchSection
	case "difftool":
		return cfg.DifftoolSection
	}
	return nil
}

func tomlToOptions(cfg *tomlConfig) types.CommonOptions {
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

// coalesce returns b if non-nil, otherwise a — a generic nil-coalescing helper
// for optional pointer fields.
func coalesce[T any](a, b *T) *T {
	if b != nil {
		return b
	}
	return a
}

// mergeOptions returns a new CommonOptions where every non-nil field in overrides
// wins over base.
func mergeOptions(base, overrides types.CommonOptions) types.CommonOptions {
	return types.CommonOptions{
		Mode:             coalesce(base.Mode, overrides.Mode),
		VCS:              coalesce(base.VCS, overrides.VCS),
		Theme:            coalesce(base.Theme, overrides.Theme),
		Keymap:           coalesce(base.Keymap, overrides.Keymap),
		AgentContext:     coalesce(base.AgentContext, overrides.AgentContext),
		Pager:            coalesce(base.Pager, overrides.Pager),
		Watch:            coalesce(base.Watch, overrides.Watch),
		ExcludeUntracked: coalesce(base.ExcludeUntracked, overrides.ExcludeUntracked),
		LineNumbers:      coalesce(base.LineNumbers, overrides.LineNumbers),
		WrapLines:        coalesce(base.WrapLines, overrides.WrapLines),
		UnkHeaders:       coalesce(base.UnkHeaders, overrides.UnkHeaders),
		AgentNotes:       coalesce(base.AgentNotes, overrides.AgentNotes),
	}
}

// normalizeLayoutMode rejects unknown layout mode strings.
func normalizeLayoutMode(s string) types.LayoutMode {
	switch types.LayoutMode(s) {
	case types.LayoutModeAuto, types.LayoutModeSplit, types.LayoutModeStack:
		return types.LayoutMode(s)
	}
	return ""
}

// normalizeVCSMode rejects unknown VCS mode strings.
func normalizeVCSMode(s string) types.VCSMode {
	switch types.VCSMode(s) {
	case types.VCSModeGit, types.VCSModeJJ:
		return types.VCSMode(s)
	}
	return ""
}

// normalizeKeymapStyle rejects unknown keymap style strings.
func normalizeKeymapStyle(s string) string {
	switch s {
	case "helix", "vim", "emacs":
		return s
	}
	return ""
}

// withOptions returns a copy of input with its options replaced.
func withOptions(input types.CLIInput, opts types.CommonOptions) types.CLIInput {
	return input.SetOptions(opts)
}
