package styles

import (
	"bytes"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kpango/unk/internal/tui/textutil"
)

// RawStyle stores pre-baked ANSI escape sequences extracted from a lipgloss.Style.
// Using RawStyle methods instead of style.Render() eliminates termenv.Profile.Color
// lookups and lipgloss getLines/strings.Split passes in tight rendering loops.
// Open and Close are exported so callers outside this package can access them directly
// for performance-critical operations like building pre-computed byte buffers.
type RawStyle struct{ Open, Close string }

// RawFromStyle extracts the ANSI open/close sequences by rendering a sentinel
// string through the style. Width is cleared so padding is not baked in.
func RawFromStyle(s lipgloss.Style) RawStyle {
	const sentinel = "\x01\x02\x03"
	open, close, found := strings.Cut(s.UnsetWidth().Render(sentinel), sentinel)
	if !found {
		return RawStyle{}
	}
	return RawStyle{Open: open, Close: close}
}

// WriteTo writes open + text + close into sb. Zero allocations.
func (r RawStyle) WriteTo(sb *bytes.Buffer, text string) {
	sb.WriteString(r.Open)
	sb.WriteString(text)
	sb.WriteString(r.Close)
}

// WriteWidthTo writes open + text (truncated to w visible columns) + padding + close.
func (r RawStyle) WriteWidthTo(sb *bytes.Buffer, text string, w int) {
	visW := textutil.VisibleWidth(text)
	if visW > w {
		text = textutil.TruncateColumns(text, w)
		visW = textutil.VisibleWidth(text)
	}
	sb.WriteString(r.Open)
	sb.WriteString(text)
	textutil.WriteSpaces(sb, w-visW)
	sb.WriteString(r.Close)
}

// WritePrefixedRawWidthTo writes open + prefix + tab-expanded text + padding + close.
func (r RawStyle) WritePrefixedRawWidthTo(sb *bytes.Buffer, prefix byte, raw string, w int) {
	r.WriteOpen(sb)
	sb.WriteByte(prefix)
	col := textutil.ExpandTabsInto(sb, raw, 1, w)
	textutil.WriteSpaces(sb, w-col)
	r.WriteClose(sb)
}

// WriteOpen writes the ANSI open sequence into sb.
func (r RawStyle) WriteOpen(sb *bytes.Buffer) { sb.WriteString(r.Open) }

// WriteClose writes the ANSI close sequence into sb.
func (r RawStyle) WriteClose(sb *bytes.Buffer) { sb.WriteString(r.Close) }
