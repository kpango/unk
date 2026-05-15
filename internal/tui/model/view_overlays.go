package model

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kpango/unk/internal/tui/textutil"
)

// overlayMenu renders a dropdown menu overlay.
func overlayMenu(base string, w, _ int, m *model) string {
	p := m.palette()
	menus := buildMenuEntries(m)

	tabStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(p.PanelAlt)).
		Foreground(lipgloss.Color(p.Muted)).
		Padding(0, 1)
	activeTabStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(p.AccentMuted)).
		Foreground(lipgloss.Color(p.Text)).
		Bold(true).
		Padding(0, 1)

	tabLabels := map[string]string{"file": "File", "view": "View", "navigate": "Navigate"}
	var menuBar strings.Builder
	for _, id := range menuIDs {
		label := tabLabels[id]
		if id == m.activeMenuID {
			menuBar.WriteString(activeTabStyle.Render(label))
		} else {
			menuBar.WriteString(tabStyle.Render(label))
		}
	}

	entries := menus[m.activeMenuID]
	itemStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(p.PanelAlt)).
		Foreground(lipgloss.Color(p.Text)).
		Padding(0, 1)
	selItemStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(p.AccentMuted)).
		Foreground(lipgloss.Color(p.Text)).
		Bold(true).
		Padding(0, 1)
	sepStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(p.PanelAlt)).
		Foreground(lipgloss.Color(p.Border)).
		Padding(0, 1)

	menuWidth := 30
	for _, e := range entries {
		l := textutil.VisibleWidth(e.label) + textutil.VisibleWidth(e.hint) + 6
		if l > menuWidth {
			menuWidth = l
		}
	}

	var dropLines []string
	for i, e := range entries {
		if e.separator {
			// colFill generates exactly (menuWidth-2) terminal cols of ─; Padding(0,1) adds 2 more.
			dropLines = append(dropLines, sepStyle.Render(textutil.ColFill('─', menuWidth-2)))
			continue
		}
		checkMark := "  "
		if e.checked != nil {
			if *e.checked {
				checkMark = "✓ "
			} else {
				checkMark = "  "
			}
		}
		cmW := textutil.VisibleWidth(checkMark)
		labelW := textutil.VisibleWidth(e.label)
		hintW := textutil.VisibleWidth(e.hint)
		pad := max(menuWidth-2-cmW-labelW-hintW, 1)
		line := checkMark + e.label + strings.Repeat(" ", pad) + e.hint
		st := itemStyle
		if i == m.menuItemIndex {
			st = selItemStyle
		}
		// Render without Width() — content is already (menuWidth-2) terminal cols; Padding(0,1) adds 2.
		dropLines = append(dropLines, st.Render(line))
	}
	dropdown := strings.Join(dropLines, "\n")

	baseLines := strings.Split(base, "\n")
	if len(baseLines) == 0 {
		return base
	}
	menuBarStr := menuBar.String()
	if len(menuBarStr) <= w && len(baseLines) > 0 {
		bl := []rune(baseLines[0])
		for j, r := range []rune(menuBarStr) {
			if j < len(bl) {
				bl[j] = r
			}
		}
		baseLines[0] = string(bl)
	}
	dropRows := strings.Split(dropdown, "\n")
	for i, row := range dropRows {
		targetRow := 1 + i
		if targetRow >= len(baseLines) {
			break
		}
		bl := []rune(baseLines[targetRow])
		rl := []rune(row)
		for j, r := range rl {
			if j < len(bl) {
				bl[j] = r
			}
		}
		baseLines[targetRow] = string(bl)
	}
	return strings.Join(baseLines, "\n")
}
