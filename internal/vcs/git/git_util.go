package git

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	gogitdiff "github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/pmezard/go-difflib/difflib"
)

// encodeTreePatch generates unified diff text between two object trees.
func encodeTreePatch(ctx context.Context, oldTree, newTree *object.Tree, pathspecs []string) (string, error) {
	var patch *object.Patch
	var err error
	if oldTree == nil {
		changes, cErr := object.DiffTreeContext(ctx, nil, newTree)
		if cErr != nil {
			return "", cErr
		}
		patch, err = changes.PatchContext(ctx)
	} else {
		patch, err = oldTree.PatchContext(ctx, newTree)
	}
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	enc := gogitdiff.NewUnifiedEncoder(&buf, 3)
	if encErr := enc.Encode(patch); encErr != nil {
		return "", encErr
	}
	text := buf.String()
	if len(pathspecs) == 0 {
		return text, nil
	}
	return filterPatchByPathspecs(text, pathspecs), nil
}

// headTreeOf returns the HEAD commit tree, or nil for an empty repo.
func headTreeOf(repo *gogit.Repository) (*object.Tree, error) {
	ref, err := repo.Head()
	if err != nil {
		return nil, err
	}
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}
	return commit.Tree()
}

// wtRoot returns the filesystem root of the repository's working tree.
func wtRoot(repo *gogit.Repository) (string, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	return wt.Filesystem.Root(), nil
}

// resolveRev resolves a human-readable revision to a hash.
func resolveRev(repo *gogit.Repository, rev string) (plumbing.Hash, error) {
	h, err := repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return *h, nil
}

// readBlob reads raw content of a blob object.
func readBlob(repo *gogit.Repository, hash plumbing.Hash) (string, error) {
	blob, err := repo.BlobObject(hash)
	if err != nil {
		return "", err
	}
	r, err := blob.Reader()
	if err != nil {
		return "", err
	}
	defer r.Close()
	var buf bytes.Buffer
	if _, err = buf.ReadFrom(r); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// formatNewFileDiff generates a new-file diff for an untracked file.
func formatNewFileDiff(filePath, content string) string {
	ud := difflib.UnifiedDiff{
		A:        nil,
		B:        difflib.SplitLines(content),
		FromFile: "/dev/null",
		ToFile:   "b/" + filePath,
		Context:  3,
	}
	body, _ := difflib.GetUnifiedDiffString(ud)
	header := fmt.Sprintf("diff --git a/%s b/%s\nnew file mode 100644\n--- /dev/null\n+++ b/%s\n",
		filePath, filePath, filePath)
	// Body from difflib includes --- and +++ lines — strip them.
	lines := strings.SplitAfter(body, "\n")
	var unkLines []string
	for _, l := range lines {
		if strings.HasPrefix(l, "---") || strings.HasPrefix(l, "+++") {
			continue
		}
		unkLines = append(unkLines, l)
	}
	return header + strings.Join(unkLines, "")
}

// patchToNumstat converts unified diff text to NUL-delimited numstat format.
func patchToNumstat(patch string) string {
	var sb strings.Builder
	var curFile string
	var adds, dels int
	flush := func() {
		if curFile != "" {
			fmt.Fprintf(&sb, "%d\t%d\t%s\x00", adds, dels, curFile)
		}
	}
	for line := range strings.SplitSeq(patch, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			curFile = ""
			adds, dels = 0, 0
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				curFile = strings.TrimPrefix(parts[3], "b/")
			}
			continue
		}
		if curFile == "" {
			continue
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			adds++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			dels++
		}
	}
	flush()
	return sb.String()
}

// pathspecMatcher returns a function that checks whether a path is included by the pathspecs.
// Handles :(exclude) magic pathspecs: ":(exclude)foo" excludes path "foo" from the result.
// Files are included when they match any positive pathspec (or no positive pathspecs exist)
// and are not matched by any exclude pathspec.
func pathspecMatcher(pathspecs []string) func(string) bool {
	if len(pathspecs) == 0 {
		return func(string) bool { return true }
	}
	var includes, excludes []string
	for _, ps := range pathspecs {
		if rest, ok := strings.CutPrefix(ps, ":(exclude)"); ok {
			excludes = append(excludes, rest)
		} else {
			includes = append(includes, ps)
		}
	}
	return func(path string) bool {
		if slices.ContainsFunc(excludes, func(ex string) bool { return pathspecPathMatch(path, ex) }) {
			return false
		}
		return len(includes) == 0 || slices.ContainsFunc(includes, func(ps string) bool { return pathspecPathMatch(path, ps) })
	}
}

// pathspecPathMatch reports whether path matches a single (non-magic) pathspec.
func pathspecPathMatch(path, spec string) bool {
	return path == spec || strings.HasPrefix(path, spec+"/") || strings.HasPrefix(spec, path+"/")
}

// filterPatchByPathspecs keeps only diff blocks whose path matches a pathspec.
func filterPatchByPathspecs(patch string, pathspecs []string) string {
	match := pathspecMatcher(pathspecs)
	var sb strings.Builder
	for _, cunk := range splitDiffBlocks(patch) {
		// Extract path from "diff --git a/<x> b/<x>".
		for _, line := range strings.SplitN(cunk, "\n", 2) {
			if strings.HasPrefix(line, "diff --git ") {
				parts := strings.Fields(line)
				if len(parts) >= 4 {
					p := strings.TrimPrefix(parts[3], "b/")
					if match(p) {
						sb.WriteString(cunk)
					}
				}
				break
			}
		}
	}
	return sb.String()
}

// splitDiffBlocks splits a multi-file patch at "diff --git " boundaries.
func splitDiffBlocks(patch string) []string {
	var blocks []string
	start := 0
	for {
		idx := strings.Index(patch[start:], "\ndiff --git ")
		if idx == -1 {
			if start < len(patch) {
				blocks = append(blocks, patch[start:])
			}
			break
		}
		end := start + idx + 1
		blocks = append(blocks, patch[start:end])
		start = end
	}
	return blocks
}
