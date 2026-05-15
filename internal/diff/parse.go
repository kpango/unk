// Package diff parses unified diff text into unk's internal model.
package diff

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kpango/unk/internal/concurrent"
	"github.com/kpango/unk/internal/types"
)

// ParseResult holds one parsed file diff with derived metadata.
type ParseResult struct {
	Metadata types.DiffMetadata
	Patch    string // the original patch text slice for this file
}

// ParsePatchFiles splits a multi-file unified diff into per-file ParseResults.
// Cunks are parsed concurrently for large diffs.
func ParsePatchFiles(patch string) []ParseResult {
	cunks := splitPatchIntoFileCunks(patch)
	if len(cunks) == 0 {
		return nil
	}
	// Parse all cunks in parallel; concurrent.Map preserves order.
	mapped := concurrent.Map(cunks, func(cunk string) (ParseResult, error) {
		r := parseOneFilePatch(cunk)
		if r == nil {
			return ParseResult{}, fmt.Errorf("skip")
		}
		return *r, nil
	})
	return concurrent.Collect(mapped)
}

// --- strip helpers ---

var terminalControlRe = regexp.MustCompile(
	`\x1bP[\s\S]*?\x1b\\` +
		`|\x1b\][\s\S]*?(?:\x07|\x1b\\)` +
		`|\x1b\[[0-?]*[ -/]*[@-~]` +
		`|\x1b[@-_]`,
)

// StripTerminalControl removes ANSI/VT escape sequences from diff text.
func StripTerminalControl(text string) string {
	return terminalControlRe.ReplaceAllString(text, "")
}

// StripGitLogMetadata removes git log commit headers (commit/Author/Date lines and
// commit-message body) from `git log -p` output, preserving only the diff content.
// Uses a stateful line-by-line scan to handle multi-paragraph commit messages
// that a single regex cannot cover.
func StripGitLogMetadata(text string) string {
	lines := strings.SplitAfter(text, "\n")
	var out []string
	inHeader := false
	for _, line := range lines {
		bare := strings.TrimRight(line, "\n")
		if !inHeader {
			if isGitCommitLine(bare) {
				inHeader = true
				continue
			}
			out = append(out, line)
			continue
		}
		// Inside a commit header block: keep lines that start the diff.
		if strings.HasPrefix(bare, "diff --git ") ||
			strings.HasPrefix(bare, "--- ") ||
			strings.HasPrefix(bare, "+++ ") {
			inHeader = false
			out = append(out, line)
		}
		// All other lines (Author, Date, Merge, blank, indented message) are dropped.
	}
	return strings.Join(out, "")
}

// NormalizeGitPatchPrefixes rewrites non-standard (mnemonic or absent) diff
// prefixes to the canonical a/ b/ form that the diff parser expects.
//
// The rewrite is scoped to diff-block header lines only: once the `+++ ` line
// has been emitted for a block the flag is cleared, so a removed line whose
// content starts with `--- ` (e.g. a SQL/Lua/Haskell comment) inside a unk
// body is not mistaken for a file header.
func NormalizeGitPatchPrefixes(text string) string {
	if !strings.Contains(text, "diff --git ") {
		return text
	}

	lines := strings.SplitAfter(text, "\n")
	var sb strings.Builder
	sb.Grow(len(text))

	// inBlock: we are inside a diff --git block (haven't yet flushed after +++).
	// sawPlusPlus: we have already emitted the +++ line for this block;
	//   after that, --- lines are unk-body content and must not be rewritten.
	inBlock := false
	sawPlusPlus := false

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			inBlock = true
			sawPlusPlus = false
			sb.WriteString(normalizeDiffGitLine(line))
		case inBlock && !sawPlusPlus && strings.HasPrefix(line, "--- "):
			rest := line[4:]
			sb.WriteString("--- ")
			sb.WriteString(normalizePathPrefix(rest, "a/"))
		case inBlock && !sawPlusPlus && strings.HasPrefix(line, "+++ "):
			rest := line[4:]
			sb.WriteString("+++ ")
			sb.WriteString(normalizePathPrefix(rest, "b/"))
			sawPlusPlus = true
		default:
			sb.WriteString(line)
		}
	}
	return sb.String()
}

// IsProbablyBinary inspects up to 8000 bytes for binary signals.
func IsProbablyBinary(data []byte) bool {
	if len(data) > 8000 {
		data = data[:8000]
	}
	total := len(data)
	if total == 0 {
		return false
	}
	// Single pass: null byte → definitely binary; count binary-signal bytes for ≥30% threshold.
	// Byte set: 0x00-0x06, 0x0E-0x1F, DEL (0x7F).
	var control int
	for _, b := range data {
		if b == 0 {
			return true
		}
		if b < 0x07 || (b > 0x0D && b < 0x20) || b == 0x7F {
			control++
		}
	}
	return control*100/total >= 30
}

// PatchLooksBinary returns true for patches with a "Binary files differ" marker.
// Uses line-anchored matching to avoid false positives from unk content.
func PatchLooksBinary(patch string) bool {
	if strings.Contains(patch, "\nGIT binary patch\n") {
		return true
	}
	// Check for "Binary files ... differ" on its own line.
	for line := range strings.SplitSeq(patch, "\n") {
		if strings.HasPrefix(line, "Binary files ") && strings.HasSuffix(line, " differ") {
			return true
		}
	}
	return false
}
