package patch

// patch_count.go — line counting helpers for positioning before unk headers.

import (
	"strings"

	"github.com/kpango/unk/internal/tui/textutil"
)

// LinesBeforeUnk counts rendered lines in the patch before the n-th @@ header.
// Correct for unified layout where every patch element renders as exactly one
// screen line (including blank elements from trailing "\n").
func LinesBeforeUnk(patch string, unkIndex int) int {
	count := 0
	hi := -1
	for {
		i := strings.IndexByte(patch, '\n')
		var line string
		if i < 0 {
			line = patch
		} else {
			line = patch[:i]
			patch = patch[i+1:]
		}
		if len(line) > 0 && line[0] == '@' {
			hi++
			if hi == unkIndex {
				return count
			}
		}
		count++
		if i < 0 {
			break
		}
	}
	return count
}

// LinesBeforeUnkSplit counts rendered lines before the n-th @@ header in split view.
//
// Split view differs from unified in two ways:
//  1. Del/add blocks are paired: N dels + M adds → max(N,M) rows, saving min(N,M) rows.
//  2. When showUnkHeaders=false, @@ header lines produce 0 rows (not 1).
func LinesBeforeUnkSplit(patch string, unkIndex int, showUnkHeaders bool) int {
	sp, lines := textutil.AcquirePatchLines(patch)
	defer textutil.ReleasePatchLines(sp)
	count := 0
	savings := 0
	unksBefore := 0
	i := 0
	for i < len(lines) {
		line := lines[i]
		if len(line) > 0 && line[0] == '@' {
			if unksBefore == unkIndex {
				adj := -savings
				if !showUnkHeaders {
					adj -= unksBefore
				}
				return count + adj
			}
			unksBefore++
			count++
			i++
			continue
		}
		if len(line) > 0 && line[0] == '-' {
			dels := 0
			for i < len(lines) && len(lines[i]) > 0 && lines[i][0] == '-' {
				dels++
				count++
				i++
			}
			adds := 0
			for i < len(lines) && len(lines[i]) > 0 && lines[i][0] == '+' {
				adds++
				count++
				i++
			}
			if dels > 0 && adds > 0 {
				savings += min(dels, adds)
			}
			continue
		}
		count++
		i++
	}
	adj := -savings
	if !showUnkHeaders {
		adj -= unksBefore
	}
	return count + adj
}

// LinesBeforeUnkStack counts rendered lines before the n-th @@ header in stack view.
//
// Stack view differs from unified:
//  1. Empty elements (trailing "\n" artifact and mid-patch blank lines) are SKIPPED.
//  2. A separator line is INSERTED between del and add blocks for each balanced unk.
//  3. When showUnkHeaders=false, the @@ header lines themselves are NOT rendered.
func LinesBeforeUnkStack(patch string, unkIndex int, showUnkHeaders bool) int {
	sp, lines := textutil.AcquirePatchLines(patch)
	defer textutil.ReleasePatchLines(sp)
	count := 0
	emptyBefore := 0
	sepsBefore := 0
	unksBefore := 0
	hasDel, hasAdd := false, false

	for _, line := range lines {
		if len(line) > 0 && line[0] == '@' {
			if hasDel && hasAdd {
				sepsBefore++
			}
			if unksBefore == unkIndex {
				adj := sepsBefore - emptyBefore
				if !showUnkHeaders {
					adj -= unksBefore
				}
				return count + adj
			}
			hasDel, hasAdd = false, false
			unksBefore++
		} else if line == "" {
			emptyBefore++
		} else if len(line) > 0 {
			switch line[0] {
			case '-':
				hasDel = true
			case '+':
				hasAdd = true
			}
		}
		count++
	}
	adj := sepsBefore - emptyBefore
	if !showUnkHeaders {
		adj -= unksBefore
	}
	return count + adj
}

// SplitViewLineSavings returns the number of lines saved in split view vs unified
// by pairing del/add blocks: each block of N dels + M adds renders as max(N,M) rows
// instead of N+M, saving min(N,M) rows.
func SplitViewLineSavings(patch string) int {
	savings := 0
	firstByte := func() byte {
		if patch == "" {
			return 0
		}
		return patch[0]
	}
	advanceLine := func() {
		if i := strings.IndexByte(patch, '\n'); i >= 0 {
			patch = patch[i+1:]
		} else {
			patch = ""
		}
	}
	for patch != "" {
		if firstByte() != '-' {
			advanceLine()
			continue
		}
		dels := 0
		for patch != "" && firstByte() == '-' {
			dels++
			advanceLine()
		}
		adds := 0
		for patch != "" && firstByte() == '+' {
			adds++
			advanceLine()
		}
		if dels > 0 && adds > 0 {
			savings += min(dels, adds)
		}
	}
	return savings
}

// StackViewLineAdjust returns the line-count adjustment for the stack layout mode
// relative to the raw fileSectionLineCount:
//
//	adjustment = separators - emptyLines [- nUnks when showUnkHeaders=false]
//
// separators are added between balanced del+add blocks; empty patch elements are
// skipped in stack rendering; @@ header lines are hidden when showUnkHeaders=false.
// Add this to FileSectionLineCount(f) to get the actual rendered line count.
func StackViewLineAdjust(patch string, showUnkHeaders bool) int {
	nUnks, separators, emptyCount := 0, 0, 0
	hasDel, hasAdd := false, false
	for {
		i := strings.IndexByte(patch, '\n')
		var line string
		if i < 0 {
			line = patch
			patch = ""
		} else {
			line = patch[:i]
			patch = patch[i+1:]
		}
		if line == "" {
			emptyCount++
		} else {
			switch line[0] {
			case '@':
				if nUnks > 0 && hasDel && hasAdd {
					separators++
				}
				hasDel, hasAdd = false, false
				nUnks++
			case '-':
				hasDel = true
			case '+':
				hasAdd = true
			}
		}
		if i < 0 {
			break
		}
	}
	if nUnks > 0 && hasDel && hasAdd {
		separators++
	}
	adj := separators - emptyCount
	if !showUnkHeaders {
		adj -= nUnks
	}
	return adj
}

// CountUnks counts the number of @@ header lines in a patch.
func CountUnks(patch string) int {
	n := 0
	for {
		i := strings.IndexByte(patch, '\n')
		var line string
		if i < 0 {
			line = patch
			patch = ""
		} else {
			line = patch[:i]
			patch = patch[i+1:]
		}
		if len(line) > 0 && line[0] == '@' {
			n++
		}
		if i < 0 {
			break
		}
	}
	return n
}
