package git

import (
	"fmt"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// buildPatchFromContent generates unified diff text by comparing old and new content maps.
// oldContent: path → content for "before"; newContent: path → content for "after".
// Missing in newContent = deleted; missing in oldContent = added.
// Binary files (containing null bytes) are emitted as "Binary files ... differ" markers
// so that downstream binary detection (PatchLooksBinary) works correctly.
func buildPatchFromContent(oldContent, newContent map[string]string, pathspecs []string) string {
	match := pathspecMatcher(pathspecs)
	var sb strings.Builder

	// Modifications and deletions.
	for path, old := range oldContent {
		if !match(path) {
			continue
		}
		new_, exists := newContent[path]
		if !exists {
			sb.WriteString(singleFileDiff(path, old, "", true, false))
			continue
		}
		if old == new_ {
			continue
		}
		if contentIsBinary(old) || contentIsBinary(new_) {
			sb.WriteString(binaryFileDiff(path, false, false))
			continue
		}
		sb.WriteString(singleFileDiff(path, old, new_, false, false))
	}

	// Additions.
	for path, new_ := range newContent {
		if !match(path) {
			continue
		}
		if _, inOld := oldContent[path]; inOld {
			continue
		}
		if contentIsBinary(new_) {
			sb.WriteString(binaryFileDiff(path, false, true))
			continue
		}
		sb.WriteString(singleFileDiff(path, "", new_, false, true))
	}

	return sb.String()
}

// contentIsBinary reports whether s contains null bytes, which are the clearest
// indicator of binary content and cannot appear in valid UTF-8 text.
func contentIsBinary(s string) bool {
	return strings.ContainsRune(s, 0)
}

// diffFilePaths returns the from/to path labels used in diff headers and binary markers.
func diffFilePaths(path string, deleted, added bool) (from, to string) {
	switch {
	case deleted:
		return "a/" + path, "/dev/null"
	case added:
		return "/dev/null", "b/" + path
	default:
		return "a/" + path, "b/" + path
	}
}

// gitDiffHeader returns the "diff --git …" header line(s) for a file.
func gitDiffHeader(path string, deleted, added bool) string {
	switch {
	case deleted:
		return fmt.Sprintf("diff --git a/%s b/%s\ndeleted file mode 100644\n", path, path)
	case added:
		return fmt.Sprintf("diff --git a/%s b/%s\nnew file mode 100644\n", path, path)
	default:
		return fmt.Sprintf("diff --git a/%s b/%s\n", path, path)
	}
}

// binaryFileDiff emits the git-style "Binary files ... differ" header so that
// PatchLooksBinary detects the file correctly and the TUI shows a binary placeholder.
func binaryFileDiff(path string, deleted, added bool) string {
	from, to := diffFilePaths(path, deleted, added)
	return gitDiffHeader(path, deleted, added) + fmt.Sprintf("Binary files %s and %s differ\n", from, to)
}

// singleFileDiff produces a git-style unified diff for one file.
func singleFileDiff(path, oldContent, newContent string, deleted, added bool) string {
	from, to := diffFilePaths(path, deleted, added)
	ud := difflib.UnifiedDiff{
		A:        difflib.SplitLines(oldContent),
		B:        difflib.SplitLines(newContent),
		FromFile: from,
		ToFile:   to,
		Context:  3,
	}
	body, err := difflib.GetUnifiedDiffString(ud)
	if err != nil || body == "" {
		return ""
	}
	return gitDiffHeader(path, deleted, added) + body
}
