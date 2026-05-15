package types

// PaletteConfig is the user-defined color palette.
// It mirrors styles.Palette but lives in types to avoid import cycles.
// Field names carry both yaml and toml tags so the same struct can be decoded
// from either config format.
type PaletteConfig struct {
	// Chrome / panels
	PanelAlt    string `yaml:"panel_alt"    toml:"panel_alt"`
	Panel       string `yaml:"panel"        toml:"panel"`
	Border      string `yaml:"border"       toml:"border"`
	Accent      string `yaml:"accent"       toml:"accent"`
	Text        string `yaml:"text"         toml:"text"`
	Muted       string `yaml:"muted"        toml:"muted"`
	AccentMuted string `yaml:"accent_muted" toml:"accent_muted"`

	// Diff line backgrounds
	AddedBg          string `yaml:"added_bg"           toml:"added_bg"`
	RemovedBg        string `yaml:"removed_bg"         toml:"removed_bg"`
	ContextBg        string `yaml:"context_bg"         toml:"context_bg"`
	AddedContentBg   string `yaml:"added_content_bg"   toml:"added_content_bg"`
	RemovedContentBg string `yaml:"removed_content_bg" toml:"removed_content_bg"`
	AddedSignColor   string `yaml:"added_sign_color"   toml:"added_sign_color"`
	RemovedSignColor string `yaml:"removed_sign_color" toml:"removed_sign_color"`

	// Unk header
	UnkHeaderFg string `yaml:"unk_header_fg" toml:"unk_header_fg"`

	// Line number column
	LineNumberBg string `yaml:"line_number_bg" toml:"line_number_bg"`
	LineNumberFg string `yaml:"line_number_fg" toml:"line_number_fg"`

	// File/sidebar change badges
	BadgeAdded   string `yaml:"badge_added"   toml:"badge_added"`
	BadgeRemoved string `yaml:"badge_removed" toml:"badge_removed"`
	BadgeNeutral string `yaml:"badge_neutral" toml:"badge_neutral"`

	// Per-change-type file colors (sidebar icon color)
	FileNew       string `yaml:"file_new"       toml:"file_new"`
	FileDeleted   string `yaml:"file_deleted"   toml:"file_deleted"`
	FileRenamed   string `yaml:"file_renamed"   toml:"file_renamed"`
	FileModified  string `yaml:"file_modified"  toml:"file_modified"`
	FileUntracked string `yaml:"file_untracked" toml:"file_untracked"`

	// Agent notes
	NoteBorder          string `yaml:"note_border"            toml:"note_border"`
	NoteBackground      string `yaml:"note_background"        toml:"note_background"`
	NoteTitleBackground string `yaml:"note_title_background"  toml:"note_title_background"`
	NoteTitleText       string `yaml:"note_title_text"        toml:"note_title_text"`
}

// KeyBindingOverrides maps snake_case action names to key sequences.
// Example: {"next_unk": ["]"], "scroll_down": ["j", "down"]}
type KeyBindingOverrides map[string][]string
