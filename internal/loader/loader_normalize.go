package loader

import (
	"fmt"
	"strings"
	"time"

	agentpkg "github.com/kpango/unk/internal/agent"
	"github.com/kpango/unk/internal/diff"
	"github.com/kpango/unk/internal/types"
)

// normalizePatchChangeset is the shared pipeline for all text-based patch sources.
func normalizePatchChangeset(patchText, title, sourceLabel string, agentCtx *types.AgentContext) *types.Changeset {
	// Normalise line endings and strip terminal escapes.
	patchText = strings.ReplaceAll(patchText, "\r\n", "\n")
	patchText = diff.StripTerminalControl(patchText)
	patchText = diff.StripGitLogMetadata(patchText)
	patchText = diff.NormalizeGitPatchPrefixes(patchText)

	cs := emptyChangeset(fmt.Sprintf("changeset:%d", time.Now().UnixMilli()))
	cs.Title = title
	cs.SourceLabel = sourceLabel

	results := diff.ParsePatchFiles(patchText)
	if len(results) == 0 {
		// Keep an empty changeset; the TUI will show an empty state.
		cs.Summary = types.Ptr("(empty changeset)")
		return cs
	}

	for i, r := range results {
		id := fmt.Sprintf("%s:%d:%s", cs.ID, i, r.Metadata.Name)
		fileCtx := agentpkg.FindFileContext(agentCtx, r.Metadata.Name, types.Deref(r.Metadata.PrevName))

		f := types.DiffFile{
			ID:       id,
			Path:     r.Metadata.Name,
			Patch:    r.Patch,
			Metadata: r.Metadata,
			Stats:    countStats(r.Metadata),
			Agent:    fileCtx,
			Language: detectLanguage(r.Metadata.Name),
		}
		if r.Metadata.PrevName != nil {
			f.PreviousPath = r.Metadata.PrevName
		}
		if diff.PatchLooksBinary(r.Patch) {
			f.IsBinary = true
		}
		cs.Files = append(cs.Files, f)
	}

	if agentCtx != nil && agentCtx.Summary != nil {
		cs.AgentSummary = agentCtx.Summary
	}
	return cs
}

// placeholderMetadata builds the shared DiffMetadata for oversized/binary placeholder entries.
func placeholderMetadata(path, cacheKeySuffix string) types.DiffMetadata {
	return types.DiffMetadata{
		Name:      path,
		Type:      types.FileChangeChange,
		IsPartial: true,
		CacheKey:  path + ":" + cacheKeySuffix,
	}
}

func buildTooLargeFile(path, changesetID string, stats types.DiffStats) types.DiffFile {
	return types.DiffFile{
		ID:             fmt.Sprintf("%s:large:%s", changesetID, path),
		Path:           path,
		Metadata:       placeholderMetadata(path, "large-diff-skipped"),
		Language:       detectLanguage(path),
		Stats:          stats,
		IsTooLarge:     true,
		StatsTruncated: true,
	}
}

func buildBinaryPlaceholder(path, changesetID string, index int) types.DiffFile {
	return types.DiffFile{
		ID:       fmt.Sprintf("%s:%d:%s", changesetID, index, path),
		Path:     path,
		Metadata: placeholderMetadata(path, "binary-skipped"),
		IsBinary: true,
	}
}

func buildUntrackedDiffFile(path, patch, changesetID string, index int, agentCtx *types.AgentContext) types.DiffFile {
	// Normalize the patch headers for untracked files before parsing.
	// git diff --no-index /dev/null <path> emits "diff --git a//dev/null b/<path>"
	// which can confuse the parser. Rewrite to a canonical form.
	patch = normalizeUntrackedPatchHeaders(path, patch)

	results := diff.ParsePatchFiles(patch)
	id := fmt.Sprintf("%s:%d:%s", changesetID, index, path)

	var meta types.DiffMetadata
	if len(results) > 0 {
		meta = results[0].Metadata
	} else {
		meta = types.DiffMetadata{Name: path, Type: types.FileChangeNew}
	}

	isBinary := diff.PatchLooksBinary(patch)
	fileCtx := agentpkg.FindFileContext(agentCtx, path, "")
	return types.DiffFile{
		ID:          id,
		Path:        path,
		Patch:       patch,
		Metadata:    meta,
		Stats:       countStats(meta),
		Agent:       fileCtx,
		Language:    detectLanguage(path),
		IsUntracked: true,
		IsBinary:    isBinary,
	}
}

// normalizeUntrackedPatchHeaders rewrites the patch headers emitted by
// "git diff --no-index /dev/null <path>" into the canonical form expected by the
// diff parser.  Specifically it ensures:
//   - The "diff --git" header uses "a/<path> b/<path>" instead of "a//dev/null b/<path>"
//   - Binary file lines are rewritten so the path is safe for the parser
func normalizeUntrackedPatchHeaders(path, patch string) string {
	canonicalHeader := "diff --git a/" + path + " b/" + path
	lines := strings.SplitAfter(patch, "\n")
	for i, line := range lines {
		stripped := strings.TrimRight(line, "\r\n")
		// Rewrite "diff --git a//dev/null b/<path>" → canonical header.
		if strings.HasPrefix(stripped, "diff --git ") {
			lines[i] = canonicalHeader + line[len(stripped):]
			continue
		}
		// Rewrite "Binary files /dev/null and <path> differ" to reference the relative path.
		if strings.HasPrefix(stripped, "Binary files /dev/null and ") {
			suffix := " differ"
			if strings.HasSuffix(stripped, suffix) {
				lines[i] = "Binary files a/" + path + " and b/" + path + suffix + line[len(stripped):]
			}
		}
	}
	return strings.Join(lines, "")
}

func emptyChangeset(id string) *types.Changeset {
	return &types.Changeset{ID: id, Files: []types.DiffFile{}}
}

// countStats sums addition/deletion lines from parsed metadata.
func countStats(meta types.DiffMetadata) types.DiffStats {
	return types.DiffStats{
		Additions: len(meta.AdditionLines),
		Deletions: len(meta.DeletionLines),
	}
}
