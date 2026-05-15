package types

// CommonOptions holds all user-configurable view settings.
// All fields are optional pointers so unset values don't override config layers.
type CommonOptions struct {
	Mode             *LayoutMode `json:"mode,omitempty"`
	VCS              *VCSMode    `json:"vcs,omitempty"`
	Theme            *string     `json:"theme,omitempty"`
	Keymap           *string     `json:"keymap,omitempty"`
	AgentContext     *string     `json:"agentContext,omitempty"`
	Pager            *bool       `json:"pager,omitempty"`
	Watch            *bool       `json:"watch,omitempty"`
	ExcludeUntracked *bool       `json:"excludeUntracked,omitempty"`
	LineNumbers      *bool       `json:"lineNumbers,omitempty"`
	WrapLines        *bool       `json:"wrapLines,omitempty"`
	UnkHeaders       *bool       `json:"unkHeaders,omitempty"`
	AgentNotes       *bool       `json:"agentNotes,omitempty"`
}

// ViewPreferences are the concrete view settings after all config layers
// are merged, with built-in defaults applied.
type ViewPreferences struct {
	Mode            LayoutMode `json:"mode"`
	Theme           *string    `json:"theme,omitempty"`
	ShowLineNumbers bool       `json:"showLineNumbers"`
	WrapLines       bool       `json:"wrapLines"`
	ShowUnkHeaders  bool       `json:"showUnkHeaders"`
	ShowAgentNotes  bool       `json:"showAgentNotes"`
}

// Bootstrap carries all startup parameters from CLI parsing into the TUI.
type Bootstrap struct {
	Input                  CLIInput           `json:"input"`
	Changeset              Changeset          `json:"changeset"`
	InitialMode            LayoutMode         `json:"initialMode"`
	InitialTheme           *string            `json:"initialTheme,omitempty"`
	InitialThemeMode       *TerminalThemeMode `json:"initialThemeMode,omitempty"`
	InitialKeymap          *string            `json:"initialKeymap,omitempty"`
	InitialShowLineNumbers *bool              `json:"initialShowLineNumbers,omitempty"`
	InitialWrapLines       *bool              `json:"initialWrapLines,omitempty"`
	InitialShowUnkHeaders  *bool              `json:"initialShowUnkHeaders,omitempty"`
	InitialShowAgentNotes  *bool              `json:"initialShowAgentNotes,omitempty"`

	// RepoRoot is the absolute path of the VCS repo root, used for session selection.
	// Empty for non-VCS inputs (file compare, patch).
	RepoRoot string `json:"repoRoot,omitempty"`
	// Version is the installed binary version string (from build-time ldflags).
	Version string `json:"version,omitempty"`

	// KeyBindingOverrides holds per-action key overrides merged from all config
	// layers. Applied on top of the base keymap selected by InitialKeymap.
	KeyBindingOverrides KeyBindingOverrides `json:"keyBindingOverrides,omitempty"`
	// CustomPalettes holds user-defined color palettes merged from all config
	// layers. Names defined here extend (and override) the built-in palette set.
	CustomPalettes map[string]PaletteConfig `json:"customPalettes,omitempty"`
}

// CLIInput is the common interface for all diff/review command inputs.
// GetOptions returns the options payload; SetOptions returns a copy of the
// input with the options replaced. Both methods use value semantics so the
// original input is never mutated.
type CLIInput interface {
	Kind() string
	GetOptions() CommonOptions
	SetOptions(CommonOptions) CLIInput
}

// OptionsOf returns the CommonOptions for any CLIInput, or a zero value for nil.
// Delegates to input.GetOptions(); kept for call-site convenience.
func OptionsOf(input CLIInput) CommonOptions {
	if input == nil {
		return CommonOptions{}
	}
	return input.GetOptions()
}

// OptionCarrier holds CommonOptions and promotes GetOptions to satisfy CLIInput.
// Embed in concrete input types to eliminate the repeated Options field and GetOptions method.
// JSON: the embedded Options field is promoted to the top level (same layout as a direct field).
type OptionCarrier struct{ Options CommonOptions }

func (c OptionCarrier) GetOptions() CommonOptions { return c.Options }
