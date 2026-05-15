package textutil

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	grepHlStart = "\x1b[7m"
	grepHlReset = "\x1b[27m"
)

const inlineHlN = 8

// InlineHlCache is a tiny N-slot FIFO cache for pre-built highlighted cells.
// Embed by value in render structs for zero-alloc per-render L1 caching.
type InlineHlCache struct {
	n     int
	text  [inlineHlN]string
	codeW [inlineHlN]int
	cell  [inlineHlN]string
}

// Get looks up (text, codeW) and returns the prebuilt cell string on hit.
func (c *InlineHlCache) Get(text string, codeW int) (string, bool) {
	n := c.n
	for i := range n {
		if c.codeW[i] == codeW && c.text[i] == text {
			return c.cell[i], true
		}
	}
	return "", false
}

// Set stores a (text, codeW) → cell entry. FIFO eviction when full.
func (c *InlineHlCache) Set(text string, codeW int, cell string) {
	if c.n < inlineHlN {
		i := c.n
		c.text[i] = text
		c.codeW[i] = codeW
		c.cell[i] = cell
		c.n++
		return
	}
	copy(c.text[:], c.text[1:])
	copy(c.codeW[:], c.codeW[1:])
	copy(c.cell[:], c.cell[1:])
	i := inlineHlN - 1
	c.text[i] = text
	c.codeW[i] = codeW
	c.cell[i] = cell
}

// OverlayGrepHighlight wraps regex matches in ansiLine with reverse-video ANSI
// markers, preserving existing escape sequences. Uses sift's regexp-based approach:
// re.FindAllStringIndex on the ANSI-stripped plain text to locate match spans,
// then re-inserts markers while replaying the original ANSI byte stream.
func OverlayGrepHighlight(ansiLine string, re *regexp.Regexp) string {
	if re == nil || ansiLine == "" {
		return ansiLine
	}
	plain := StripANSI(ansiLine)
	allIdx := re.FindAllStringIndex(plain, -1)
	if len(allIdx) == 0 {
		return ansiLine
	}

	type span struct{ lo, hi int }
	spans := make([]span, len(allIdx))
	for i, idx := range allIdx {
		spans[i] = span{idx[0], idx[1]}
	}

	sb := AcquireBuilder()
	defer ReleaseBuilder(sb)
	sb.Grow(len(ansiLine) + len(spans)*(len(grepHlStart)+len(grepHlReset)))

	plainPos := 0
	si := 0
	inHL := false

	j := 0
	for j < len(ansiLine) {
		if ansiLine[j] == '\x1b' && j+1 < len(ansiLine) && ansiLine[j+1] == '[' {
			end := j + 2
			for end < len(ansiLine) && ansiLine[end] != 'm' {
				end++
			}
			if end < len(ansiLine) {
				end++
			}
			sb.WriteString(ansiLine[j:end])
			j = end
			continue
		}

		if si < len(spans) {
			if !inHL && plainPos == spans[si].lo {
				sb.WriteString(grepHlStart)
				inHL = true
			} else if inHL && plainPos == spans[si].hi {
				sb.WriteString(grepHlReset)
				inHL = false
				si++
				if si < len(spans) && plainPos == spans[si].lo {
					sb.WriteString(grepHlStart)
					inHL = true
				}
			}
		}

		_, size := utf8.DecodeRuneInString(ansiLine[j:])
		sb.WriteString(ansiLine[j : j+size])
		j += size
		plainPos += size
	}
	if inHL {
		sb.WriteString(grepHlReset)
	}
	return sb.String()
}

// ApplyGrepHighlightToSection applies grep highlighting to every line of a
// pre-rendered ANSI section. Returns section unchanged when no matches.
func ApplyGrepHighlightToSection(section string, re *regexp.Regexp) string {
	if re == nil || section == "" {
		return section
	}
	sb := AcquireBuilder()
	defer ReleaseBuilder(sb)
	sb.Grow(len(section) + 64)
	changed := false
	start := 0
	for {
		end := strings.IndexByte(section[start:], '\n')
		var line string
		if end < 0 {
			line = section[start:]
		} else {
			line = section[start : start+end]
		}
		hl := OverlayGrepHighlight(line, re)
		if hl != line {
			changed = true
		}
		sb.WriteString(hl)
		if end < 0 {
			break
		}
		sb.WriteByte('\n')
		start += end + 1
	}
	if !changed {
		return section
	}
	return sb.String()
}
