package styles

import "github.com/kpango/unk/internal/types"

// RegisterCustomPalettes merges user-defined palettes into the global Palettes
// map and registers their names in the theme cycle. Custom names override
// built-ins if they collide. Call once at startup before the TUI renders.
func RegisterCustomPalettes(custom map[string]types.PaletteConfig) {
	for name, pc := range custom {
		Palettes[name] = paletteConfigToPalette(pc)
		addCustomThemeName(name)
	}
}

// paletteConfigToPalette converts a user-facing PaletteConfig to a Palette.
// Empty strings in the config are left as empty (the renderer treats them as
// "no color", which lipgloss renders as the terminal default).
func paletteConfigToPalette(pc types.PaletteConfig) Palette {
	return Palette{
		PanelAlt:    pc.PanelAlt,
		Panel:       pc.Panel,
		Border:      pc.Border,
		Accent:      pc.Accent,
		Text:        pc.Text,
		Muted:       pc.Muted,
		AccentMuted: pc.AccentMuted,

		AddedBg:          pc.AddedBg,
		RemovedBg:        pc.RemovedBg,
		ContextBg:        pc.ContextBg,
		AddedContentBg:   pc.AddedContentBg,
		RemovedContentBg: pc.RemovedContentBg,
		AddedSignColor:   pc.AddedSignColor,
		RemovedSignColor: pc.RemovedSignColor,

		UnkHeaderFg: pc.UnkHeaderFg,

		LineNumberBg: pc.LineNumberBg,
		LineNumberFg: pc.LineNumberFg,

		BadgeAdded:   pc.BadgeAdded,
		BadgeRemoved: pc.BadgeRemoved,
		BadgeNeutral: pc.BadgeNeutral,

		FileNew:       pc.FileNew,
		FileDeleted:   pc.FileDeleted,
		FileRenamed:   pc.FileRenamed,
		FileModified:  pc.FileModified,
		FileUntracked: pc.FileUntracked,

		NoteBorder:          pc.NoteBorder,
		NoteBackground:      pc.NoteBackground,
		NoteTitleBackground: pc.NoteTitleBackground,
		NoteTitleText:       pc.NoteTitleText,
	}
}

// Palettes is the built-in named color palettes.
var Palettes = map[string]Palette{
	"graphite": {
		PanelAlt:    "#1d2126",
		Panel:       "#171a1d",
		Border:      "#343c45",
		Accent:      "#d5e0ea",
		Text:        "#f2f4f6",
		Muted:       "#9aa4af",
		AccentMuted: "#414a54",

		AddedBg:          "#1f3025",
		RemovedBg:        "#372526",
		ContextBg:        "#181c20",
		AddedContentBg:   "#24362a",
		RemovedContentBg: "#432b2d",
		AddedSignColor:   "#88d39b",
		RemovedSignColor: "#f0a0a0",

		UnkHeaderFg: "#9aa4af",

		LineNumberBg: "#14181b",
		LineNumberFg: "#798592",

		BadgeAdded:   "#88d39b",
		BadgeRemoved: "#f0a0a0",
		BadgeNeutral: "#a9b4bf",

		FileNew:       "#88d39b",
		FileDeleted:   "#f0a0a0",
		FileRenamed:   "#e6cf98",
		FileModified:  "#c49bff",
		FileUntracked: "#7fd1ff",

		NoteBorder:          "#c6a0ff",
		NoteBackground:      "#241c31",
		NoteTitleBackground: "#322446",
		NoteTitleText:       "#f5edff",
	},
	"midnight": {
		PanelAlt:    "#13243a",
		Panel:       "#0e1b2e",
		Border:      "#284264",
		Accent:      "#7fd1ff",
		Text:        "#eef4ff",
		Muted:       "#8da5c7",
		AccentMuted: "#355578",

		AddedBg:          "#153526",
		RemovedBg:        "#47262a",
		ContextBg:        "#0f1b2d",
		AddedContentBg:   "#102a1f",
		RemovedContentBg: "#371b1e",
		AddedSignColor:   "#69d69a",
		RemovedSignColor: "#ff8e8e",

		UnkHeaderFg: "#8da5c7",

		LineNumberBg: "#0b1627",
		LineNumberFg: "#56739a",

		BadgeAdded:   "#5ad188",
		BadgeRemoved: "#ff8b8b",
		BadgeNeutral: "#89a5d3",

		FileNew:       "#5ad188",
		FileDeleted:   "#ff8b8b",
		FileRenamed:   "#ffd883",
		FileModified:  "#b794f6",
		FileUntracked: "#7fd1ff",

		NoteBorder:          "#c49bff",
		NoteBackground:      "#211a36",
		NoteTitleBackground: "#30234f",
		NoteTitleText:       "#f5eeff",
	},
	"paper": {
		PanelAlt:    "#f8f1e7",
		Panel:       "#fffaf3",
		Border:      "#d8c8b3",
		Accent:      "#77593a",
		Text:        "#2f2417",
		Muted:       "#786753",
		AccentMuted: "#d7ccbe",

		AddedBg:          "#dff0e1",
		RemovedBg:        "#f6ddde",
		ContextBg:        "#faf6ee",
		AddedContentBg:   "#eaf8ec",
		RemovedContentBg: "#fbebeb",
		AddedSignColor:   "#3f8d58",
		RemovedSignColor: "#b4545b",

		UnkHeaderFg: "#786753",

		LineNumberBg: "#f2e9dc",
		LineNumberFg: "#9b8367",

		BadgeAdded:   "#3f8d58",
		BadgeRemoved: "#b4545b",
		BadgeNeutral: "#8e7355",

		FileNew:       "#3f8d58",
		FileDeleted:   "#b4545b",
		FileRenamed:   "#9f6c1f",
		FileModified:  "#7d5bc4",
		FileUntracked: "#4a6890",

		NoteBorder:          "#7d5bc4",
		NoteBackground:      "#efe6ff",
		NoteTitleBackground: "#e3d7ff",
		NoteTitleText:       "#462b74",
	},
	"ember": {
		PanelAlt:    "#2c1710",
		Panel:       "#22120d",
		Border:      "#643627",
		Accent:      "#ffb07a",
		Text:        "#fff0e6",
		Muted:       "#c7a18d",
		AccentMuted: "#5d3428",

		AddedBg:          "#183424",
		RemovedBg:        "#4a1f1f",
		ContextBg:        "#24140e",
		AddedContentBg:   "#21432c",
		RemovedContentBg: "#5a2727",
		AddedSignColor:   "#83d99d",
		RemovedSignColor: "#ff9d8f",

		UnkHeaderFg: "#c7a18d",

		LineNumberBg: "#1c100c",
		LineNumberFg: "#9a735f",

		BadgeAdded:   "#83d99d",
		BadgeRemoved: "#ff9d8f",
		BadgeNeutral: "#f1be9d",

		FileNew:       "#83d99d",
		FileDeleted:   "#ff9d8f",
		FileRenamed:   "#ffd08f",
		FileModified:  "#d8b4fe",
		FileUntracked: "#ffb07a",

		NoteBorder:          "#e1a3ff",
		NoteBackground:      "#311d36",
		NoteTitleBackground: "#452650",
		NoteTitleText:       "#fff0ff",
	},
	"catppuccin-mocha": {
		PanelAlt:    "#1e1e2e",
		Panel:       "#181825",
		Border:      "#45475a",
		Accent:      "#89b4fa",
		Text:        "#cdd6f4",
		Muted:       "#6c7086",
		AccentMuted: "#313244",

		AddedBg:          "#1e2d1e",
		RemovedBg:        "#2d1e1e",
		ContextBg:        "#1a1a2e",
		AddedContentBg:   "#253525",
		RemovedContentBg: "#352525",
		AddedSignColor:   "#a6e3a1",
		RemovedSignColor: "#f38ba8",

		UnkHeaderFg: "#89b4fa",

		LineNumberBg: "#11111b",
		LineNumberFg: "#585b70",

		BadgeAdded:   "#a6e3a1",
		BadgeRemoved: "#f38ba8",
		BadgeNeutral: "#a6adc8",

		FileNew:       "#a6e3a1",
		FileDeleted:   "#f38ba8",
		FileRenamed:   "#f9e2af",
		FileModified:  "#cba6f7",
		FileUntracked: "#89b4fa",

		NoteBorder:          "#cba6f7",
		NoteBackground:      "#1e1b16",
		NoteTitleBackground: "#28253a",
		NoteTitleText:       "#f9e2af",
	},
	"catppuccin-latte": {
		PanelAlt:    "#eff1f5",
		Panel:       "#dce0e8",
		Border:      "#bcc0cc",
		Accent:      "#1e66f5",
		Text:        "#4c4f69",
		Muted:       "#9ca0b0",
		AccentMuted: "#ccd0da",

		AddedBg:          "#e6f3e6",
		RemovedBg:        "#f3e6e6",
		ContextBg:        "#e8ecf0",
		AddedContentBg:   "#d5ecd5",
		RemovedContentBg: "#ecd5d5",
		AddedSignColor:   "#40a02b",
		RemovedSignColor: "#d20f39",

		UnkHeaderFg: "#1e66f5",

		LineNumberBg: "#e6e9ef",
		LineNumberFg: "#acb0be",

		BadgeAdded:   "#40a02b",
		BadgeRemoved: "#d20f39",
		BadgeNeutral: "#7c7f93",

		FileNew:       "#40a02b",
		FileDeleted:   "#d20f39",
		FileRenamed:   "#df8e1d",
		FileModified:  "#8839ef",
		FileUntracked: "#1e66f5",

		NoteBorder:          "#8839ef",
		NoteBackground:      "#f3f0e3",
		NoteTitleBackground: "#ede8d0",
		NoteTitleText:       "#df8e1d",
	},
	"solarized-dark": {
		PanelAlt:    "#073642",
		Panel:       "#002b36",
		Border:      "#0a4c5e",
		Accent:      "#268bd2",
		Text:        "#839496",
		Muted:       "#586e75",
		AccentMuted: "#083d4f",

		AddedBg:          "#002b00",
		RemovedBg:        "#2b0000",
		ContextBg:        "#01262e",
		AddedContentBg:   "#003b00",
		RemovedContentBg: "#3b0000",
		AddedSignColor:   "#859900",
		RemovedSignColor: "#dc322f",

		UnkHeaderFg: "#268bd2",

		LineNumberBg: "#001c24",
		LineNumberFg: "#4e6064",

		BadgeAdded:   "#859900",
		BadgeRemoved: "#dc322f",
		BadgeNeutral: "#657b83",

		FileNew:       "#859900",
		FileDeleted:   "#dc322f",
		FileRenamed:   "#b58900",
		FileModified:  "#268bd2",
		FileUntracked: "#2aa198",

		NoteBorder:          "#6c71c4",
		NoteBackground:      "#001c30",
		NoteTitleBackground: "#002644",
		NoteTitleText:       "#eee8d5",
	},
	"solarized-light": {
		PanelAlt:    "#eee8d5",
		Panel:       "#fdf6e3",
		Border:      "#d0c8a8",
		Accent:      "#268bd2",
		Text:        "#657b83",
		Muted:       "#93a1a1",
		AccentMuted: "#e3ddc5",

		AddedBg:          "#f0f4e0",
		RemovedBg:        "#f4e0e0",
		ContextBg:        "#f7f1dc",
		AddedContentBg:   "#e0ecc0",
		RemovedContentBg: "#ecc0c0",
		AddedSignColor:   "#859900",
		RemovedSignColor: "#dc322f",

		UnkHeaderFg: "#268bd2",

		LineNumberBg: "#f7f1de",
		LineNumberFg: "#9da8a3",

		BadgeAdded:   "#859900",
		BadgeRemoved: "#dc322f",
		BadgeNeutral: "#839496",

		FileNew:       "#859900",
		FileDeleted:   "#dc322f",
		FileRenamed:   "#b58900",
		FileModified:  "#268bd2",
		FileUntracked: "#2aa198",

		NoteBorder:          "#6c71c4",
		NoteBackground:      "#fdf0d0",
		NoteTitleBackground: "#f5e8b0",
		NoteTitleText:       "#b58900",
	},
}
