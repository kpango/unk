package textutil

// textutil_linenum.go — line number width computation helpers.

import (
	"strings"
)

// ComputeLineNumWidth returns the column count needed to render the largest line number in lines.
func ComputeLineNumWidth(lines []string) int {
	maxNum := 0
	oldLine, newLine := 0, 0
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case '@':
			// ParseUnkHeader is in the patch package; replicate inline to avoid circular import.
			// Scan for old start after '-' and new start after '+'.
			o, n := parseUnkHeaderLineNum(line)
			if o > 0 {
				oldLine = o - 1
			}
			if n > 0 {
				newLine = n - 1
			}
		case '+':
			newLine++
			if newLine > maxNum {
				maxNum = newLine
			}
		case '-':
			oldLine++
			if oldLine > maxNum {
				maxNum = oldLine
			}
		default:
			oldLine++
			newLine++
			if newLine > maxNum {
				maxNum = newLine
			}
		}
	}
	if maxNum < 10 {
		return 3
	}
	digits := 0
	for n := maxNum; n > 0; n /= 10 {
		digits++
	}
	return digits + 1
}

// parseUnkHeaderLineNum extracts old and new start line numbers from a @@ header line.
// Minimal version used by ComputeLineNumWidth to avoid an import cycle with tui/patch.
func parseUnkHeaderLineNum(line string) (oldStart, newStart int) {
	i := strings.IndexByte(line, '-')
	if i < 0 {
		return -1, -1
	}
	i++
	old, ok := scanDecIntTU(line, &i)
	if !ok {
		return -1, -1
	}
	j := strings.IndexByte(line, '+')
	if j < 0 {
		return -1, -1
	}
	j++
	newVal, ok2 := scanDecIntTU(line, &j)
	if !ok2 {
		return -1, -1
	}
	return old, newVal
}

// scanDecIntTU reads a decimal integer from s starting at *pos, advancing *pos past the digits.
func scanDecIntTU(s string, pos *int) (int, bool) {
	n := 0
	start := *pos
	for *pos < len(s) {
		c := s[*pos]
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
		*pos++
	}
	return n, *pos > start
}
