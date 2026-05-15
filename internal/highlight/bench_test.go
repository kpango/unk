package highlight_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/kpango/unk/internal/highlight"
)

var benchLines = []string{
	`func processRequest(ctx context.Context, req *http.Request) (*Response, error) {`,
	`	if err := validateRequest(req); err != nil {`,
	`		return nil, fmt.Errorf("invalid request: %w", err)`,
	`	}`,
	`	resp, err := h.client.Do(ctx, req)`,
	`	if err != nil {`,
	`		return nil, err`,
	`	}`,
	`	return parseResponse(resp)`,
	`}`,
}

// BenchmarkLine measures cached highlight lookups cycling through 10 distinct lines.
func BenchmarkLine(b *testing.B) {
	style := highlight.ForTheme("graphite")
	bg := lipgloss.Color("#1f3025")
	// Warm the line cache.
	for _, l := range benchLines {
		_ = highlight.Line(l, "go", bg, style)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := b.N; i > 0; i-- {
		_ = highlight.Line(benchLines[i%len(benchLines)], "go", bg, style)
	}
}

// BenchmarkLineCold measures the full tokenize+render path (no line cache hit).
func BenchmarkLineCold(b *testing.B) {
	bg := lipgloss.Color("#1f3025")
	b.ReportAllocs()
	b.ResetTimer()
	for i := b.N; i > 0; i-- {
		// Each iteration creates a fresh Style so the line cache is empty.
		style := highlight.ForTheme("graphite")
		_ = highlight.Line(benchLines[i%len(benchLines)], "go", bg, style)
	}
}

// BenchmarkLineRepeat measures repeated rendering of the same line (cache saturated).
func BenchmarkLineRepeat(b *testing.B) {
	style := highlight.ForTheme("graphite")
	bg := lipgloss.Color("#1f3025")
	line := benchLines[0]
	_ = highlight.Line(line, "go", bg, style) // warm
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = highlight.Line(line, "go", bg, style)
	}
}
