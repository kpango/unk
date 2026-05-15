package highlight

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestForTheme(t *testing.T) {
	themes := []string{"graphite", "catppuccin-mocha", "catppuccin-latte", "solarized-dark", "solarized-light", "unknown"}
	for _, th := range themes {
		s := ForTheme(th)
		if s == nil {
			t.Errorf("ForTheme(%q) returned nil", th)
		}
		// Repeated calls should return the same cached instance.
		if ForTheme(th) != s {
			t.Errorf("ForTheme(%q) not cached", th)
		}
	}
}

func TestLineKnownLanguage(t *testing.T) {
	style := ForTheme("graphite")
	result := Line("func main() {}", "go", lipgloss.Color("#003300"), style)
	if result == "" {
		t.Fatal("Line returned empty for known language 'go'")
	}
	// Result should contain the original text somewhere (sans ANSI codes).
	stripped := stripANSI(result)
	if !strings.Contains(stripped, "func") {
		t.Errorf("stripped result %q doesn't contain 'func'", stripped)
	}
	if !strings.Contains(stripped, "main") {
		t.Errorf("stripped result %q doesn't contain 'main'", stripped)
	}
}

func TestLineUnknownLanguage(t *testing.T) {
	style := ForTheme("graphite")
	if got := Line("some text", "notareallanguage", lipgloss.Color(""), style); got != "" {
		t.Errorf("Line with unknown language should return empty, got %q", got)
	}
}

func TestLineEmptyInputs(t *testing.T) {
	style := ForTheme("graphite")
	if got := Line("", "go", lipgloss.Color(""), style); got != "" {
		t.Error("Line with empty text should return empty")
	}
	if got := Line("code", "", lipgloss.Color(""), style); got != "" {
		t.Error("Line with empty langID should return empty")
	}
	if got := Line("code", "go", lipgloss.Color(""), nil); got != "" {
		t.Error("Line with nil style should return empty")
	}
}

func TestLineProducesOutput(t *testing.T) {
	// Verify that highlighting a multi-token Go expression produces a non-empty result
	// with the original text content preserved (stripping ANSI codes).
	style := ForTheme("graphite")
	result := Line("x := 1 + 2", "go", lipgloss.Color("#001122"), style)
	if result == "" {
		t.Fatal("Line returned empty for a valid Go expression")
	}
	stripped := stripANSI(result)
	for _, tok := range []string{"x", "1", "2"} {
		if !strings.Contains(stripped, tok) {
			t.Errorf("stripped output %q missing token %q", stripped, tok)
		}
	}
}

// stripANSI removes ANSI escape sequences from s for comparison.
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			for i < len(s) && (s[i] < '@' || s[i] > '~') {
				i++
			}
			if i < len(s) {
				i++ // consume the final byte of the CSI sequence
			}
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
