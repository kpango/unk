package styles

import (
	"bytes"

	"github.com/charmbracelet/lipgloss"

	"github.com/kpango/unk/internal/syncmap"
)

// Palette defines the color palette used by all rendering functions.
type Palette struct {
	// Chrome / panels
	PanelAlt string // menu bar, status bar background
	Panel    string // file header, context line background
	Border   string // divider lines, separators
	Accent   string // primary accent (selection strip, active border)

	// Text
	Text  string // primary text
	Muted string // secondary / context text

	// Interactive
	AccentMuted string // active/selected item background (menus, dropdowns)

	// Diff line backgrounds
	AddedBg          string // addition line background
	RemovedBg        string // deletion line background
	ContextBg        string // context (unchanged) line background
	AddedContentBg   string // intra-line changed span background (addition side)
	RemovedContentBg string // intra-line changed span background (deletion side)

	// Diff sign/text colors
	AddedSignColor   string // addition marker and text foreground
	RemovedSignColor string // deletion marker and text foreground

	// Unk header
	UnkHeaderFg string

	// Line number column
	LineNumberBg string
	LineNumberFg string

	// File/sidebar change badges
	BadgeAdded   string
	BadgeRemoved string
	BadgeNeutral string

	// Per-change-type file colors (sidebar icon color)
	FileNew       string
	FileDeleted   string
	FileRenamed   string
	FileModified  string
	FileUntracked string

	// Agent notes
	NoteBorder          string
	NoteBackground      string
	NoteTitleBackground string
	NoteTitleText       string
}

// RendererStyles holds every lipgloss.Style needed by the rendering pipeline,
// pre-computed once per theme palette.
type RendererStyles struct {
	// ── Menu bar ──────────────────────────────────────────────────────────
	MenuBar   lipgloss.Style
	MenuTitle lipgloss.Style
	MenuLabel lipgloss.Style
	MenuAdd   lipgloss.Style
	MenuDel   lipgloss.Style

	// ── Scrollbar ─────────────────────────────────────────────────────────
	ScrollBg        lipgloss.Style
	ScrollTrack     lipgloss.Style
	ScrollThumb     lipgloss.Style
	ScrollThumbDrag lipgloss.Style

	// ── File-section header ────────────────────────────────────────────────
	FileHeader lipgloss.Style
	FileAdd    lipgloss.Style
	FileDel    lipgloss.Style

	// ── Diff-line styles ──────────────────────────────────────────────────
	DiffAdd lipgloss.Style
	DiffDel lipgloss.Style
	DiffCtx lipgloss.Style
	DiffHH  lipgloss.Style
	LineNum lipgloss.Style

	// ── Intra-line span highlights ────────────────────────────────────────
	IntraAdd lipgloss.Style
	IntraDel lipgloss.Style

	// ── Separators / borders ──────────────────────────────────────────────
	Border lipgloss.Style

	// ── Sidebar: base variants ────────────────────────────────────────────
	SbGroup    lipgloss.Style
	SbRowBase  lipgloss.Style
	SbRowSel   lipgloss.Style
	SbMuted    lipgloss.Style
	SbMutedSel lipgloss.Style
	SbAdd      lipgloss.Style
	SbAddSel   lipgloss.Style
	SbDel      lipgloss.Style
	SbDelSel   lipgloss.Style
	SbStrip    lipgloss.Style
	SbStripOff lipgloss.Style
	SbRowWrap    lipgloss.Style
	SbRowWrapSel lipgloss.Style
	SbEmpty      lipgloss.Style
	SbIconNew       [2]lipgloss.Style
	SbIconDeleted   [2]lipgloss.Style
	SbIconRenamed   [2]lipgloss.Style
	SbIconModified  [2]lipgloss.Style
	SbIconUntracked [2]lipgloss.Style

	// ── Status bar ────────────────────────────────────────────────────────
	StatusBar lipgloss.Style

	// ── Menu overlay ──────────────────────────────────────────────────────
	OverlayTab        lipgloss.Style
	OverlayTabActive  lipgloss.Style
	OverlayItem       lipgloss.Style
	OverlayItemActive lipgloss.Style

	// ── Agent-note box ────────────────────────────────────────────────────
	NoteBorder lipgloss.Style
	NoteTitle  lipgloss.Style
	NoteBody   lipgloss.Style

	// ── Raw ANSI variants for zero-alloc hot-loop rendering ──────────────
	RawLineNum    RawStyle
	RawDiffAdd    RawStyle
	RawDiffDel    RawStyle
	RawDiffCtx    RawStyle
	RawDiffHH     RawStyle
	RawIntraAdd   RawStyle
	RawIntraDel   RawStyle
	RawFileHeader RawStyle
	RawFileAdd    RawStyle
	RawFileDel    RawStyle
	RawBorder     RawStyle
	DivStr        string
	RawSbStrip    RawStyle
	RawSbStripOff RawStyle
	RawSbRowBase  RawStyle
	RawSbRowSel   RawStyle
	RawSbMuted    RawStyle
	RawSbMutedSel RawStyle
	RawSbAdd      RawStyle
	RawSbAddSel   RawStyle
	RawSbDel      RawStyle
	RawSbDelSel   RawStyle
	RawSbEmpty    RawStyle
	RawSbGroup    RawStyle
	RawSbIcons    [5][2]RawStyle
	RawMenuBar    RawStyle
	RawMenuTitle  RawStyle
	RawMenuAdd    RawStyle
	RawMenuDel    RawStyle
	MenuLabels    string
	MenuLabelsW   int
	RawStatusBar  RawStyle
	RawScrollBg        RawStyle
	RawScrollTrack     RawStyle
	RawScrollThumb     RawStyle
	RawScrollThumbDrag RawStyle
	ScrollTrackStr     string
	ScrollThumbStr     string
	ScrollThumbDragStr string
}

// RendererStylesCache stores one *RendererStyles per theme ID.
var RendererStylesCache = syncmap.New[string, *RendererStyles]()

// ComputeRendererStyles builds the full style set from a theme palette.
func ComputeRendererStyles(p Palette) *RendererStyles {
	b := func(bg string) lipgloss.Style {
		return lipgloss.NewStyle().Background(lipgloss.Color(bg))
	}
	f := func(fg string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(fg))
	}
	bf := func(bg, fg string) lipgloss.Style {
		return lipgloss.NewStyle().Background(lipgloss.Color(bg)).Foreground(lipgloss.Color(fg))
	}
	icon2 := func(norm, sel string) [2]lipgloss.Style {
		return [2]lipgloss.Style{
			bf(p.Panel, norm),
			bf(p.PanelAlt, sel),
		}
	}

	rs := &RendererStyles{
		MenuBar:   bf(p.PanelAlt, p.Text),
		MenuTitle: bf(p.PanelAlt, p.Text).Bold(true).Padding(0, 1),
		MenuLabel: bf(p.PanelAlt, p.Muted).Padding(0, 1),
		MenuAdd:   bf(p.PanelAlt, p.BadgeAdded),
		MenuDel:   bf(p.PanelAlt, p.BadgeRemoved),

		ScrollBg:        b(p.PanelAlt),
		ScrollTrack:     bf(p.PanelAlt, p.Border),
		ScrollThumb:     bf(p.PanelAlt, p.Accent),
		ScrollThumbDrag: bf(p.PanelAlt, p.Text),

		FileHeader: bf(p.Panel, p.Text).Bold(true),
		FileAdd:    bf(p.Panel, p.BadgeAdded),
		FileDel:    bf(p.Panel, p.BadgeRemoved),

		DiffAdd: bf(p.AddedBg, p.AddedSignColor),
		DiffDel: bf(p.RemovedBg, p.RemovedSignColor),
		DiffCtx: bf(p.ContextBg, p.Muted),
		DiffHH:  bf(p.ContextBg, p.UnkHeaderFg),
		LineNum: bf(p.LineNumberBg, p.LineNumberFg),

		IntraAdd: bf(p.AddedContentBg, p.AddedSignColor),
		IntraDel: bf(p.RemovedContentBg, p.RemovedSignColor),

		Border: f(p.Border),

		SbGroup:    bf(p.Panel, p.Muted),
		SbRowBase:  bf(p.Panel, p.Text),
		SbRowSel:   bf(p.PanelAlt, p.Text),
		SbMuted:    bf(p.Panel, p.Muted),
		SbMutedSel: bf(p.PanelAlt, p.Muted),
		SbAdd:      bf(p.Panel, p.BadgeAdded),
		SbAddSel:   bf(p.PanelAlt, p.BadgeAdded),
		SbDel:      bf(p.Panel, p.BadgeRemoved),
		SbDelSel:   bf(p.PanelAlt, p.BadgeRemoved),

		SbStrip:    b(p.Accent),
		SbStripOff: b(p.Panel),

		SbRowWrap:    b(p.Panel),
		SbRowWrapSel: b(p.PanelAlt),
		SbEmpty:      b(p.Panel),

		SbIconNew:       icon2(p.FileNew, p.FileNew),
		SbIconDeleted:   icon2(p.FileDeleted, p.FileDeleted),
		SbIconRenamed:   icon2(p.FileRenamed, p.FileRenamed),
		SbIconModified:  icon2(p.FileModified, p.FileModified),
		SbIconUntracked: icon2(p.FileUntracked, p.FileUntracked),

		StatusBar: bf(p.PanelAlt, p.Muted).Padding(0, 1),

		OverlayTab:        bf(p.PanelAlt, p.Muted).Padding(0, 1),
		OverlayTabActive:  bf(p.AccentMuted, p.Text).Bold(true).Padding(0, 1),
		OverlayItem:       bf(p.Panel, p.Text).Padding(0, 1),
		OverlayItemActive: bf(p.AccentMuted, p.Text).Padding(0, 1),

		NoteBorder: f(p.NoteBorder),
		NoteTitle:  bf(p.NoteTitleBackground, p.NoteTitleText),
		NoteBody:   bf(p.NoteBackground, p.NoteTitleText),
	}

	rs.RawLineNum = RawFromStyle(rs.LineNum)
	rs.RawDiffAdd = RawFromStyle(rs.DiffAdd)
	rs.RawDiffDel = RawFromStyle(rs.DiffDel)
	rs.RawDiffCtx = RawFromStyle(rs.DiffCtx)
	rs.RawDiffHH = RawFromStyle(rs.DiffHH)
	rs.RawIntraAdd = RawFromStyle(rs.IntraAdd)
	rs.RawIntraDel = RawFromStyle(rs.IntraDel)
	rs.RawFileHeader = RawFromStyle(rs.FileHeader)
	rs.RawFileAdd = RawFromStyle(rs.FileAdd)
	rs.RawFileDel = RawFromStyle(rs.FileDel)
	rs.RawBorder = RawFromStyle(rs.Border)
	rs.DivStr = rs.Border.Render("│")
	rs.RawSbStrip = RawFromStyle(rs.SbStrip)
	rs.RawSbStripOff = RawFromStyle(rs.SbStripOff)
	rs.RawSbRowBase = RawFromStyle(rs.SbRowBase)
	rs.RawSbRowSel = RawFromStyle(rs.SbRowSel)
	rs.RawSbMuted = RawFromStyle(rs.SbMuted)
	rs.RawSbMutedSel = RawFromStyle(rs.SbMutedSel)
	rs.RawSbAdd = RawFromStyle(rs.SbAdd)
	rs.RawSbAddSel = RawFromStyle(rs.SbAddSel)
	rs.RawSbDel = RawFromStyle(rs.SbDel)
	rs.RawSbDelSel = RawFromStyle(rs.SbDelSel)
	rs.RawSbEmpty = RawFromStyle(rs.SbEmpty)
	rs.RawSbGroup = RawFromStyle(rs.SbGroup)
	rs.RawSbIcons[0] = [2]RawStyle{RawFromStyle(rs.SbIconNew[0]), RawFromStyle(rs.SbIconNew[1])}
	rs.RawSbIcons[1] = [2]RawStyle{RawFromStyle(rs.SbIconDeleted[0]), RawFromStyle(rs.SbIconDeleted[1])}
	rs.RawSbIcons[2] = [2]RawStyle{RawFromStyle(rs.SbIconRenamed[0]), RawFromStyle(rs.SbIconRenamed[1])}
	rs.RawSbIcons[3] = [2]RawStyle{RawFromStyle(rs.SbIconUntracked[0]), RawFromStyle(rs.SbIconUntracked[1])}
	rs.RawSbIcons[4] = [2]RawStyle{RawFromStyle(rs.SbIconModified[0]), RawFromStyle(rs.SbIconModified[1])}

	rs.RawMenuBar = RawFromStyle(rs.MenuBar)
	rs.RawMenuTitle = RawFromStyle(rs.MenuTitle)
	rs.RawMenuAdd = RawFromStyle(rs.MenuAdd)
	rs.RawMenuDel = RawFromStyle(rs.MenuDel)
	var menuLabelsBuf bytes.Buffer
	rawMenuLabel := RawFromStyle(rs.MenuLabel)
	rawMenuLabel.WriteTo(&menuLabelsBuf, "File")
	rawMenuLabel.WriteTo(&menuLabelsBuf, "View")
	rawMenuLabel.WriteTo(&menuLabelsBuf, "Navigate")
	rs.MenuLabels = menuLabelsBuf.String()
	rs.MenuLabelsW = lipgloss.Width(rs.MenuLabels)
	rs.RawStatusBar = RawFromStyle(rs.StatusBar)

	rs.RawScrollBg = RawFromStyle(rs.ScrollBg)
	rs.RawScrollTrack = RawFromStyle(rs.ScrollTrack)
	rs.RawScrollThumb = RawFromStyle(rs.ScrollThumb)
	rs.RawScrollThumbDrag = RawFromStyle(rs.ScrollThumbDrag)
	var scrollBuf bytes.Buffer
	rs.RawScrollTrack.WriteTo(&scrollBuf, "│")
	rs.ScrollTrackStr = scrollBuf.String()
	scrollBuf.Reset()
	rs.RawScrollThumb.WriteTo(&scrollBuf, "█")
	rs.ScrollThumbStr = scrollBuf.String()
	scrollBuf.Reset()
	rs.RawScrollThumbDrag.WriteTo(&scrollBuf, "█")
	rs.ScrollThumbDragStr = scrollBuf.String()

	return rs
}

// SidebarIconRaw returns the pre-baked RawStyle for the given icon color and
// selection state (sel=0 normal, sel=1 selected). Zero allocs.
func SidebarIconRaw(iconColor string, sel int, p Palette, rs *RendererStyles) RawStyle {
	switch iconColor {
	case p.FileNew:
		return rs.RawSbIcons[0][sel]
	case p.FileDeleted:
		return rs.RawSbIcons[1][sel]
	case p.FileRenamed:
		return rs.RawSbIcons[2][sel]
	case p.FileUntracked:
		return rs.RawSbIcons[3][sel]
	default:
		return rs.RawSbIcons[4][sel]
	}
}
