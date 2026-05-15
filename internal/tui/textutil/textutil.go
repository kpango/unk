package textutil

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// VisibleWidth returns the number of terminal columns occupied by s.
// For all-ASCII text this is O(n) with no allocations; non-ASCII uses runewidth.
func VisibleWidth(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return runewidth.StringWidth(s)
		}
	}
	return len(s)
}

// ExpandTabs replaces tab characters with spaces using 4-column tab stops.
func ExpandTabs(s string) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	buf := AcquireBuilder()
	defer ReleaseBuilder(buf)
	buf.Grow(len(s) * 2)
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := 4 - (col % 4)
			WriteSpaces(buf, spaces)
			col += spaces
		} else {
			buf.WriteRune(r)
			col += runewidth.RuneWidth(r)
		}
	}
	return buf.String()
}

// ExpandTabsInto writes s with tabs expanded directly into sb starting at col.
// Stops when the next character would exceed maxCols. Returns final column.
//
// Fast path: all-ASCII, no tabs, fits within maxCols → single WriteString.
// Normal path: batches consecutive ASCII non-tab runs into single WriteString calls,
// falling back to per-rune handling only for tabs or multibyte characters.
func ExpandTabsInto(sb *bytes.Buffer, s string, col, maxCols int) int {
	// Fast path: all ASCII, no tabs, fits entirely within maxCols.
	remaining := maxCols - col
	if remaining >= len(s) {
		for i := 0; i < len(s); i++ {
			if s[i] == '\t' || s[i] >= utf8.RuneSelf {
				goto slow
			}
		}
		sb.WriteString(s)
		return col + len(s)
	}
	// Fast path: same but truncated at remaining cols (all ASCII, no tabs).
	{
		hasSpecial := false
		for i := 0; i < len(s); i++ {
			if s[i] == '\t' || s[i] >= utf8.RuneSelf {
				hasSpecial = true
				break
			}
		}
		if !hasSpecial {
			if remaining > 0 {
				sb.WriteString(s[:remaining])
			}
			return maxCols
		}
	}

slow:
	// Batch-write consecutive ASCII non-tab runs; handle tabs and multibyte individually.
	batchStart := 0
	i := 0
	for i < len(s) {
		b := s[i]
		if b >= utf8.RuneSelf {
			// Flush pending ASCII batch.
			if i > batchStart {
				sb.WriteString(s[batchStart:i])
			}
			r, size := utf8.DecodeRuneInString(s[i:])
			rw := runewidth.RuneWidth(r)
			if col+rw > maxCols {
				return col
			}
			sb.WriteString(s[i : i+size])
			col += rw
			i += size
			batchStart = i
		} else if b == '\t' {
			// Flush pending ASCII batch.
			if i > batchStart {
				sb.WriteString(s[batchStart:i])
			}
			spaces := 4 - (col & 3)
			if col+spaces > maxCols {
				spaces = maxCols - col
			}
			if spaces <= 0 {
				return col
			}
			WriteSpaces(sb, spaces)
			col += spaces
			i++
			batchStart = i
		} else {
			// ASCII non-tab: accumulate in batch.
			if col+1 > maxCols {
				if i > batchStart {
					sb.WriteString(s[batchStart:i])
				}
				return col
			}
			col++
			i++
		}
	}
	// Flush any remaining ASCII batch.
	if i > batchStart {
		sb.WriteString(s[batchStart:i])
	}
	return col
}

// SkipColumns returns s with the first cols visual columns removed.
func SkipColumns(s string, cols int) string {
	if cols <= 0 {
		return s
	}
	allASCII := true
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			allASCII = false
			break
		}
	}
	if allASCII {
		if cols >= len(s) {
			return ""
		}
		return s[cols:]
	}
	accumulated := 0
	for i, r := range s {
		if accumulated >= cols {
			return s[i:]
		}
		accumulated += runewidth.RuneWidth(r)
	}
	return ""
}

// TruncateColumns returns s truncated to at most cols terminal columns.
func TruncateColumns(s string, cols int) string {
	if cols <= 0 {
		return ""
	}
	allASCII := true
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			allASCII = false
			break
		}
	}
	if allASCII {
		if len(s) > cols {
			return s[:cols]
		}
		return s
	}
	accumulated := 0
	for i, r := range s {
		w := runewidth.RuneWidth(r)
		if accumulated+w > cols {
			return s[:i]
		}
		accumulated += w
	}
	return s
}

// TruncateTabAware truncates s to at most cols terminal columns (tabs as 4-col stops).
// Returns both the truncated string and its exact column width.
func TruncateTabAware(s string, cols int) (string, int) {
	if cols <= 0 || len(s) == 0 {
		return "", 0
	}
	if len(s) <= cols {
		for i := 0; i < len(s); i++ {
			b := s[i]
			if b == '\t' || b > 127 {
				goto slow
			}
		}
		return s, len(s)
	}
slow:
	col := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		var w int
		switch {
		case c == '\t':
			w = 4 - (col & 3)
		case c <= 127:
			w = 1
		default:
			for j, r := range s[i:] {
				if r == '\t' {
					w = 4 - (col & 3)
				} else {
					w = runewidth.RuneWidth(r)
				}
				if col+w > cols {
					return s[:i+j], col
				}
				col += w
			}
			return s, col
		}
		if col+w > cols {
			return s[:i], col
		}
		col += w
	}
	return s, col
}

// TruncateNameSuffix returns the suffix of s that fits in cols terminal columns,
// scanning from the end. Used for sidebar filenames to show the tail (basename).
func TruncateNameSuffix(s string, cols int) string {
	if cols <= 0 {
		return ""
	}
	runes := []rune(s)
	acc := 0
	for i := len(runes) - 1; i >= 0; i-- {
		w := runewidth.RuneWidth(runes[i])
		if acc+w > cols {
			return string(runes[i+1:])
		}
		acc += w
	}
	return s
}

// StripANSI removes ANSI SGR escape sequences from s, returning plain text.
func StripANSI(s string) string {
	if !strings.ContainsRune(s, '\x1b') {
		return s
	}
	sb := AcquireBuilder()
	defer ReleaseBuilder(sb)
	sb.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		sb.WriteByte(s[i])
		i++
	}
	return sb.String()
}

// SplitLines splits s on '\n', appending each segment to buf (no copies).
func SplitLines(s string, buf []string) []string {
	for {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			return append(buf, s)
		}
		buf = append(buf, s[:i])
		s = s[i+1:]
	}
}

// BuildLineOffsets returns a []int32 of byte-start positions per line in s.
func BuildLineOffsets(s string) []int32 {
	if len(s) == 0 {
		return nil
	}
	offsets := make([]int32, 1, len(s)/64+2)
	offsets[0] = 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' && i+1 < len(s) {
			offsets = append(offsets, int32(i+1))
		}
	}
	return offsets
}

// CountDigits returns the number of decimal digits needed to represent n.
func CountDigits(n int) int {
	if n <= 0 {
		return 1
	}
	d := 0
	for ; n > 0; n /= 10 {
		d++
	}
	return d
}

// ApplyHorizontalOffset trims the first offset runes from s (zero-alloc range iteration).
func ApplyHorizontalOffset(s string, offset int) string {
	if offset <= 0 {
		return s
	}
	count := 0
	for i := range s {
		if count >= offset {
			return s[i:]
		}
		count++
	}
	return ""
}

// ColFill returns a string of rune ch repeated to fill exactly cols terminal columns.
func ColFill(ch rune, cols int) string {
	if cols <= 0 {
		return ""
	}
	w := runewidth.RuneWidth(ch)
	if w <= 0 {
		w = 1
	}
	n := cols / w
	s := strings.Repeat(string(ch), n)
	if rem := cols % w; rem > 0 {
		s += strings.Repeat(" ", rem)
	}
	return s
}
