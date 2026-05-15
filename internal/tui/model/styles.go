package model

import (
	"github.com/kpango/unk/internal/tui/styles"
)

// styles returns the pre-computed RendererStyles for the current theme.
// Results are computed once and cached permanently in styles.RendererStylesCache.
func (m *model) styles() *styles.RendererStyles {
	if rs, ok := styles.RendererStylesCache.Load(m.themeID); ok {
		return rs
	}
	rs := styles.ComputeRendererStyles(m.palette())
	styles.RendererStylesCache.Store(m.themeID, rs)
	return rs
}

// palette returns the color palette for the current theme.
func (m *model) palette() styles.Palette {
	if p, ok := styles.Palettes[m.themeID]; ok {
		return p
	}
	return styles.Palettes["graphite"]
}
