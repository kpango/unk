package overlay

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/kpango/unk/internal/tui/keys"
	"github.com/kpango/unk/internal/tui/textutil"
)

// --- lipgloss styles ---

var (
	EmptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Align(lipgloss.Center, lipgloss.Center)

	HelpStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#111111")).
			Foreground(lipgloss.Color("#eeeeee")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#555555")).
			Padding(0, 1)

	FloatStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1c1c2e")).
			Foreground(lipgloss.Color("#cdd6f4")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#89b4fa")).
			Padding(0, 1)
)

// RenderFloatingWindow centers a pre-rendered dialog box over a dimmed version of the
// base frame. Background ANSI codes are stripped and replaced with a uniform dim color;
// compositing uses visual column widths so ANSI in the base never corrupts placement.
func RenderFloatingWindow(base string, termW, termH int, dialog string) string {
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < termH {
		baseLines = append(baseLines, "")
	}

	// Measure dialog dimensions from the rendered box.
	dialogRows := strings.Split(dialog, "\n")
	// Trim trailing empty line that lipgloss sometimes appends.
	for len(dialogRows) > 0 && dialogRows[len(dialogRows)-1] == "" {
		dialogRows = dialogRows[:len(dialogRows)-1]
	}
	dH := len(dialogRows)
	dW := 0
	for _, r := range dialogRows {
		if w := textutil.VisibleWidth(textutil.StripANSI(r)); w > dW {
			dW = w
		}
	}

	startRow := max((termH-dH)/2, 0)
	startCol := max((termW-dW)/2, 0)

	// Raw ANSI for the dimmed background: dark foreground on very dark background.
	const dimOpen = "\x1b[38;5;240m\x1b[48;5;235m"
	const dimClose = "\x1b[0m"

	sb := textutil.AcquireBuilder()
	defer textutil.ReleaseBuilder(sb)

	for i, baseLine := range baseLines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		// Strip ANSI and pad to full terminal width.
		plain := textutil.StripANSI(baseLine)
		pw := textutil.VisibleWidth(plain)
		switch {
		case pw < termW:
			plain += strings.Repeat(" ", termW-pw)
		case pw > termW:
			plain = textutil.TruncateColumns(plain, termW)
		}

		di := i - startRow
		if di < 0 || di >= dH {
			// Background-only row: full dim.
			sb.WriteString(dimOpen)
			sb.WriteString(plain)
			sb.WriteString(dimClose)
			continue
		}

		// Row overlaps dialog: dim-left | dialog-row | dim-right.
		left := textutil.TruncateColumns(plain, startCol)
		lw := textutil.VisibleWidth(left)
		leftPad := strings.Repeat(" ", startCol-lw) // pad wide-char boundary gap

		right := textutil.SkipColumns(plain, startCol+dW)

		sb.WriteString(dimOpen)
		sb.WriteString(left)
		sb.WriteString(leftPad)
		sb.WriteString(dimClose)
		sb.WriteString(dialogRows[di])
		sb.WriteString(dimOpen)
		sb.WriteString(right)
		sb.WriteString(dimClose)
	}
	return sb.String()
}

// Help renders a centered, tabbed help dialog as a floating window.
// page selects the active section: 0=Keys, 1=Commands, 2=Modes & Themes.
func Help(base string, w, h int, km keys.KeyMap, page int) string {
	hk := func(b key.Binding) string { return b.Help().Key }

	// Tab bar.
	tabNames := [3]string{"Keys", "Commands", "Modes & Themes"}
	var tabBar strings.Builder
	for i, t := range tabNames {
		if i > 0 {
			tabBar.WriteString("  ")
		}
		if i == page {
			tabBar.WriteString("● " + t)
		} else {
			tabBar.WriteString("○ " + t)
		}
	}

	var sections []string
	switch page {
	case 0: // Keys
		sections = []string{
			" " + tabBar.String(),
			"",
			" Navigation",
			fmt.Sprintf("  %-12s prev file  %-12s next file", hk(km.PrevFile), hk(km.NextFile)),
			fmt.Sprintf("  %-12s prev unk   %-12s next unk", hk(km.PrevUnk), hk(km.NextUnk)),
			fmt.Sprintf("  %-12s prev annotated unk", hk(km.PrevAnnotatedUnk)),
			fmt.Sprintf("  %-12s next annotated unk", hk(km.NextAnnotatedUnk)),
			"",
			" Scroll",
			fmt.Sprintf("  %-12s up    %-10s down", hk(km.ScrollUp), hk(km.ScrollDown)),
			fmt.Sprintf("  %-12s half-up  %-10s half-down", hk(km.ScrollUpHalf), hk(km.ScrollDownHalf)),
			fmt.Sprintf("  %-12s page-up  %-10s page-down", hk(km.ScrollPageUp), hk(km.ScrollPageDown)),
			fmt.Sprintf("  %-12s top   %-10s bottom", hk(km.ScrollTop), hk(km.ScrollBottom)),
			fmt.Sprintf("  %-12s left  %-10s right", hk(km.ScrollLeft), hk(km.ScrollRight)),
			fmt.Sprintf("  %-12s fast-← %-10s fast-→", hk(km.ScrollLeftFast), hk(km.ScrollRightFast)),
			"",
			" Layout",
			fmt.Sprintf("  %-6s split  %-6s stack  %-6s auto", hk(km.LayoutSplit), hk(km.LayoutStack), hk(km.LayoutAuto)),
			"",
			" Display",
			fmt.Sprintf("  %-6s sidebar  %-6s line numbers", hk(km.ToggleSidebar), hk(km.ToggleLineNumbers)),
			fmt.Sprintf("  %-6s wrap     %-6s unk headers", hk(km.ToggleWrap), hk(km.ToggleUnkHeaders)),
			fmt.Sprintf("  %-6s notes    %-6s cycle theme", hk(km.ToggleAgentNotes), hk(km.CycleTheme)),
			fmt.Sprintf("  %-6s sidebar focus", hk(km.ToggleFocus)),
			"",
			" Search & Filter",
			fmt.Sprintf("  %-6s filter file list", hk(km.FocusFilter)),
			fmt.Sprintf("  %-6s grep diff content", hk(km.FocusSearch)),
			fmt.Sprintf("  %-6s next match  %-6s prev match", hk(km.SearchNext), hk(km.SearchPrev)),
			"",
			" Other",
			fmt.Sprintf("  %-6s open in editor  %-6s yank unk to clipboard", hk(km.OpenInEditor), hk(km.YankUnk)),
			fmt.Sprintf("  %-6s reload   %-6s command mode", hk(km.Refresh), hk(km.CommandMode)),
			fmt.Sprintf("  %-6s help     %-6s quit", hk(km.Help), hk(km.Quit)),
			"",
			" Mouse",
			"  click sidebar  → select file",
			"  drag divider   → resize sidebar",
			"  click scrollbar→ jump / drag",
			"",
			" ← / → or Tab to switch sections",
		}

	case 1: // Commands
		sections = []string{
			" " + tabBar.String(),
			"",
			" Press : to open command mode",
			"",
			" General",
			"  :q  :quit        quit unk",
			"  :h  :help        show this help",
			"  :reload  :e      reload from VCS",
			"",
			" Layout",
			"  :split  :sp      side-by-side diff",
			"  :stack  :st      del-above/add-below",
			"  :auto            responsive auto mode",
			"",
			" Display Toggles",
			"  :wrap            toggle line wrap",
			"  :numbers  :nu    toggle line numbers",
			"  :sidebar  :sb    toggle sidebar",
			"  :headers  :hd    toggle @@ headers",
			"  :notes           toggle agent notes",
			"",
			" Theme & Keymap",
			"  :theme [name]    set or cycle theme",
			"  :keymap [style]  set or list keymaps",
			"  :km [style]      alias for :keymap",
			"",
			" File Navigation",
			"  :first           jump to first file",
			"  :last            jump to last file",
			"  :N               jump to file N (1-based)",
			"",
			" Search & Filter",
			"  :filter [text]   filter the file list",
			"  :f [text]        alias for :filter",
			"  :search [text]   grep diff content",
			"  :grep [text]     alias for :search",
			"  :g [text]        alias for :search",
			"",
			" ← / → or Tab to switch sections",
		}

	default: // Modes & Themes
		sections = []string{
			" " + tabBar.String(),
			"",
			" Layout Modes",
			"  unified   one-column diff (default)",
			"  split     side-by-side columns",
			"  stack     deletions above additions",
			"            (set via :split / :stack / :auto)",
			"",
			" Keymap Styles",
			"  helix     Helix Editor style (default)",
			"            C-u/C-d half-page, C-b/C-f page",
			"  vim       Vim style",
			"            u/d half-page, b/space/f page",
			"  emacs     Emacs style",
			"            C-p/C-n lines, M-v/C-v page",
			"            (set via :keymap <style>)",
			"",
			" Themes",
			"  graphite    midnight    paper   ember",
			"  catppuccin-mocha        catppuccin-latte",
			"  solarized-dark          solarized-light",
			"            (set via :theme <name> or t key)",
			"",
			" CLI Flags",
			"  --theme <name>      initial theme",
			"  --keymap <style>    initial keymap",
			"  --split / --stack   initial layout",
			"  --pager             hide sidebar/menu",
			"  --watch             auto-reload on change",
			"  --no-line-numbers   hide line numbers",
			"  --wrap              enable line wrap",
			"",
			" ← / → or Tab to switch sections",
		}
	}

	dialog := HelpStyle.Width(w / 2).Render(strings.Join(sections, "\n"))
	return RenderFloatingWindow(base, w, h, dialog)
}

// KeymapList renders a centered keymap-selection dialog as a floating window.
// activeStyle is the currently committed keymap; selectedIdx is the cursor position.
func KeymapList(base string, w, h int, activeStyle string, selectedIdx int) string {
	var sb strings.Builder
	sb.WriteString(" Select Keymap\n\n")
	for i, ks := range keys.KeymapStyles {
		cursor := "  "
		if i == selectedIdx {
			cursor = "> "
		}
		label := cursor + ks
		if ks == activeStyle {
			label += "  ✓"
		}
		sb.WriteString(" " + label + "\n")
	}
	sb.WriteString("\n ↑/↓  Enter  Esc")
	dialog := FloatStyle.Width(24).Render(sb.String())
	return RenderFloatingWindow(base, w, h, dialog)
}
