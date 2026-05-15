package patch

// patch_parse.go — unk header parsing and patch segment extraction.

import (
	"strings"
)

// ParseUnkHeader extracts old and new start line numbers from a @@ header line.
// Returns -1, -1 on parse failure. Uses a zero-alloc byte scanner.
func ParseUnkHeader(line string) (oldStart, newStart int) {
	i := strings.IndexByte(line, '-')
	if i < 0 {
		return -1, -1
	}
	i++
	old, ok := scanDecInt(line, &i)
	if !ok {
		return -1, -1
	}
	j := strings.IndexByte(line, '+')
	if j < 0 {
		return -1, -1
	}
	j++
	newVal, ok2 := scanDecInt(line, &j)
	if !ok2 {
		return -1, -1
	}
	return old, newVal
}

// scanDecInt reads a decimal integer from s starting at *pos, advancing *pos past the digits.
func scanDecInt(s string, pos *int) (int, bool) {
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

// NewStartLine returns the new-file start line ("+N" in the @@ header) of
// the n-th unk in patch. Returns 0 if the unk is not found.
func NewStartLine(patch string, unkIndex int) int {
	hi := -1
	for {
		nl := strings.IndexByte(patch, '\n')
		var line string
		if nl < 0 {
			line = patch
		} else {
			line = patch[:nl]
			patch = patch[nl+1:]
		}
		if len(line) > 0 && line[0] == '@' {
			hi++
			if hi == unkIndex {
				_, newStart := ParseUnkHeader(line)
				return newStart
			}
		}
		if nl < 0 {
			break
		}
	}
	return 0
}

// ExtractText returns the raw patch lines (including the @@ header) for the
// n-th unk in patch. Lines are newline-separated. Returns "" if not found.
func ExtractText(patch string, unkIndex int) string {
	hi := -1
	start := -1
	pos := 0
	for pos <= len(patch) {
		nl := strings.IndexByte(patch[pos:], '\n')
		end := pos + nl
		if nl < 0 {
			end = len(patch)
		}
		line := patch[pos:end]
		if len(line) > 0 && line[0] == '@' {
			hi++
			if hi == unkIndex {
				start = pos
			} else if hi == unkIndex+1 && start >= 0 {
				return patch[start:pos]
			}
		}
		if nl < 0 {
			break
		}
		pos = end + 1
	}
	if start >= 0 {
		return patch[start:]
	}
	return ""
}

// Segment is one unk's worth of patch lines with pre-parsed rendering state.
type Segment struct {
	Lines        []string // includes the leading @@ header line
	FirstLineIdx int      // index of Lines[0] in the original patch lines slice
	StartOld     int      // oldLine implied by the @@ header (= old_start - 1)
	StartNew     int      // newLine implied by the @@ header (= new_start - 1)
	UnkIdx       int      // 0-based unk index (for annotation lookup)
}

// SplitUnks divides a patch line slice (already split on "\n") into per-unk
// segments. Each segment begins with its @@ header line. Lines before the
// first @@ header are ignored.
func SplitUnks(lines []string) []Segment {
	nUnks := 0
	for _, l := range lines {
		if len(l) > 0 && l[0] == '@' {
			nUnks++
		}
	}
	segs := make([]Segment, 0, nUnks)
	start := -1
	unkCount := 0
	for i, l := range lines {
		if len(l) == 0 || l[0] != '@' {
			continue
		}
		if start >= 0 {
			segs[len(segs)-1].Lines = lines[start:i]
		}
		o, n := ParseUnkHeader(l)
		startOld := 0
		if o > 0 {
			startOld = o - 1
		}
		startNew := 0
		if n > 0 {
			startNew = n - 1
		}
		segs = append(segs, Segment{
			FirstLineIdx: i,
			StartOld:     startOld,
			StartNew:     startNew,
			UnkIdx:       unkCount,
		})
		start = i
		unkCount++
	}
	if start >= 0 && start < len(lines) {
		segs[len(segs)-1].Lines = lines[start:]
	}
	return segs
}
