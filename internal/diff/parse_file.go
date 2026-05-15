package diff

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strconv"
	"strings"

	"github.com/kpango/unk/internal/types"
)

var (
	// git diff header: "diff --git a/<old> b/<new>"
	gitDiffHeaderRe = regexp.MustCompile(`(?m)^diff --git a/(.*) b/(.*)$`)
	// rename: "rename from <old>" / "rename to <new>"
	renameFromRe = regexp.MustCompile(`(?m)^rename from (.+)$`)
	renameToRe   = regexp.MustCompile(`(?m)^rename to (.+)$`)
	// copy: "copy from <old>" / "copy to <new>"
	copyFromRe = regexp.MustCompile(`(?m)^copy from (.+)$`)
	copyToRe   = regexp.MustCompile(`(?m)^copy to (.+)$`)
	// new/deleted file markers with optional mode
	newFileModeRe     = regexp.MustCompile(`(?m)^new file mode (\d+)`)
	deletedFileModeRe = regexp.MustCompile(`(?m)^deleted file mode (\d+)`)
	// old/new mode for permission-only changes
	oldFileModeRe     = regexp.MustCompile(`(?m)^old mode (\d+)`)
	newFileModeOnlyRe = regexp.MustCompile(`(?m)^new mode (\d+)`)
	// unk header: "@@ -<old_start>[,<old_count>] +<new_start>[,<new_count>] @@"
	unkHeaderRe = regexp.MustCompile(`@@\s+-(\d+)(?:,(\d+))?\s+\+(\d+)(?:,(\d+))?\s+@@`)
	// "Binary files a/x and b/x differ"
	binaryRe = regexp.MustCompile(`(?m)^Binary files`)

	gitCommitSHARe = regexp.MustCompile(`^commit [0-9a-f]{4,64}(?: |$)`)
)

// splitPatchIntoFileCunks splits a multi-file patch on "diff --git " boundaries.
// Non-git patches (from `unk diff <file1> <file2>`) are split on "--- " / "+++ " pairs.
func splitPatchIntoFileCunks(patch string) []string {
	if strings.Contains(patch, "diff --git ") {
		return splitOnPrefix(patch, "diff --git ")
	}
	// Non-git patch: split on "--- " that begins a new file block.
	return splitOnPrefix(patch, "--- ")
}

func splitOnPrefix(text, prefix string) []string {
	var cunks []string
	start := 0
	for {
		idx := strings.Index(text[start:], "\n"+prefix)
		if idx == -1 {
			if start < len(text) {
				cunks = append(cunks, text[start:])
			}
			break
		}
		end := start + idx + 1 // +1 to include the newline before prefix
		cunks = append(cunks, text[start:end])
		start = end
	}
	if len(cunks) == 0 && text != "" {
		cunks = append(cunks, text)
	}
	return cunks
}

// parseOneFilePatch parses a single-file diff cunk.
func parseOneFilePatch(cunk string) *ParseResult {
	if strings.TrimSpace(cunk) == "" {
		return nil
	}

	meta := types.DiffMetadata{
		Type: types.FileChangeChange,
	}

	// Extract file names.
	if m := gitDiffHeaderRe.FindStringSubmatch(cunk); m != nil {
		meta.Name = normPath(m[2]) // new name
		if m[1] != m[2] {
			prev := normPath(m[1])
			meta.PrevName = &prev
		}
	}

	// Rename detection.
	if m := renameToRe.FindStringSubmatch(cunk); m != nil {
		meta.Name = normPath(m[1])
	}
	if m := renameFromRe.FindStringSubmatch(cunk); m != nil {
		prev := normPath(m[1])
		meta.PrevName = &prev
	}

	// Copy detection: copy from/to marks a file copy (prevName = origin).
	if m := copyToRe.FindStringSubmatch(cunk); m != nil {
		meta.Name = normPath(m[1])
	}
	if m := copyFromRe.FindStringSubmatch(cunk); m != nil {
		prev := normPath(m[1])
		meta.PrevName = &prev
	}

	// File change type and mode.
	isRename := renameToRe.MatchString(cunk)
	isCopy := copyToRe.MatchString(cunk)
	if m := newFileModeRe.FindStringSubmatch(cunk); m != nil {
		meta.Type = types.FileChangeNew
		mode := m[1]
		meta.Mode = &mode
	} else if m := deletedFileModeRe.FindStringSubmatch(cunk); m != nil {
		meta.Type = types.FileChangeDeleted
		mode := m[1]
		meta.Mode = &mode
	} else if m := oldFileModeRe.FindStringSubmatch(cunk); m != nil {
		prevMode := m[1]
		meta.PrevMode = &prevMode
		if m2 := newFileModeOnlyRe.FindStringSubmatch(cunk); m2 != nil {
			mode := m2[1]
			meta.Mode = &mode
		}
	}
	// Resolve rename/copy type after unks are counted (below).

	// Binary file.
	if binaryRe.MatchString(cunk) {
		meta.IsPartial = true
		meta.CacheKey = cacheKey(meta.Name, cunk)
		return &ParseResult{Metadata: meta, Patch: cunk}
	}

	// Parse unk headers and count lines.
	unks, addLines, delLines, splitCount, unifiedCount := parseUnks(cunk)
	meta.Unks = unks
	meta.AdditionLines = addLines
	meta.DeletionLines = delLines
	meta.SplitLineCount = splitCount
	meta.UnifiedLineCount = unifiedCount
	meta.CacheKey = cacheKey(meta.Name, cunk)

	// Resolve rename/copy change type now that we know whether unks exist.
	if isRename || isCopy {
		if len(unks) > 0 {
			meta.Type = types.FileChangeRenameChanged
		} else {
			meta.Type = types.FileChangeRenamePure
		}
	}

	// Derive name from +++ line when no git header present.
	if meta.Name == "" {
		meta.Name = extractPlusName(cunk)
	}

	return &ParseResult{Metadata: meta, Patch: cunk}
}

// parseUnks extracts all unk headers and accumulates line number lists.
func parseUnks(cunk string) (unks []types.DiffUnk, addLines, delLines []int, splitCount, unifiedCount int) {
	unkIndex := 0
	oldLine := 0
	newLine := 0

	rest := cunk
	for rest != "" {
		var line string
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			line = rest[:nl]
			rest = rest[nl+1:]
		} else {
			line = rest
			rest = ""
		}
		if m := unkHeaderRe.FindStringSubmatch(line); m != nil {
			oldStart := atoi(m[1])
			newStart := atoi(m[3])

			oldCount := 1
			if m[2] != "" {
				oldCount = atoi(m[2])
			}
			newCount := 1
			if m[4] != "" {
				newCount = atoi(m[4])
			}

			oldRange := [2]int{oldStart, oldCount}
			newRange := [2]int{newStart, newCount}
			unks = append(unks, types.DiffUnk{
				Index:    unkIndex,
				Header:   line,
				OldRange: &oldRange,
				NewRange: &newRange,
			})
			unkIndex++
			oldLine = oldStart
			newLine = newStart
			// Header row counts in unified view.
			unifiedCount++
			// In split view the header spans both sides.
			splitCount++
			continue
		}

		if len(line) == 0 {
			continue
		}

		switch line[0] {
		case '+':
			addLines = append(addLines, newLine)
			newLine++
			unifiedCount++
			splitCount++
		case '-':
			delLines = append(delLines, oldLine)
			oldLine++
			unifiedCount++
			splitCount++
		case ' ':
			newLine++
			oldLine++
			unifiedCount++
			splitCount++
		}
	}
	return
}

// extractPlusName parses the filename from the "+++ " line.
func extractPlusName(cunk string) string {
	for line := range strings.SplitSeq(cunk, "\n") {
		if name, ok := strings.CutPrefix(line, "+++ "); ok {
			name = strings.TrimPrefix(name, "b/")
			return strings.TrimRight(name, "\r\n")
		}
	}
	return ""
}

// normPath strips the a/ or b/ diff prefix.
func normPath(p string) string {
	p = strings.TrimRight(p, "\r")
	p = strings.TrimPrefix(p, "a/")
	p = strings.TrimPrefix(p, "b/")
	return p
}

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

// cacheKey produces a short deterministic key for rendered output caching
// using FNV-1a 64-bit.
func cacheKey(name, patch string) string {
	h := fnv.New64a()
	h.Write([]byte(name))
	h.Write([]byte(patch))
	return fmt.Sprintf("%s:%x", name, h.Sum64())
}

func isGitCommitLine(line string) bool {
	return gitCommitSHARe.MatchString(line)
}
