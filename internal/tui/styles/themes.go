package styles

import (
	"slices"
	"sort"
)

// builtinThemes is the ordered list of built-in theme IDs for cycling.
var builtinThemes = []string{
	"graphite",
	"midnight",
	"paper",
	"ember",
	"catppuccin-mocha",
	"catppuccin-latte",
	"solarized-dark",
	"solarized-light",
}

// customThemeNames holds user-defined theme names appended after the built-ins
// in the cycle. Populated by RegisterCustomPalettes.
var customThemeNames []string

// ThemeIDs returns all available theme IDs in cycle order: built-ins first,
// then user-defined themes sorted alphabetically.
func ThemeIDs() []string {
	if len(customThemeNames) == 0 {
		return builtinThemes
	}
	all := make([]string, 0, len(builtinThemes)+len(customThemeNames))
	all = append(all, builtinThemes...)
	all = append(all, customThemeNames...)
	return all
}

// NextTheme returns the theme ID that follows current in the cycle.
// If current is not found, returns the first theme.
func NextTheme(current string) string {
	all := ThemeIDs()
	i := slices.Index(all, current)
	if i < 0 {
		return all[0]
	}
	return all[(i+1)%len(all)]
}

// addCustomThemeName registers a custom theme name in the cycle list if it is
// not already present (either as a built-in or existing custom name).
func addCustomThemeName(name string) {
	if slices.Contains(builtinThemes, name) {
		return
	}
	if slices.Contains(customThemeNames, name) {
		return
	}
	customThemeNames = append(customThemeNames, name)
	sort.Strings(customThemeNames)
}
