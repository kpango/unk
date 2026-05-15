// Package vcs defines the interface for version control backend operations.
package vcs

import (
	"strconv"
	"strings"

	"github.com/kpango/unk/internal/types"
)

// Client is the interface all VCS backends implement.
type Client interface {
	// DiffText returns the raw unified diff text for the given input.
	DiffText(input *types.VCSInput, cwd string) (string, error)

	// ShowText returns the raw unified diff for a specific commit or ref.
	ShowText(input *types.ShowInput, cwd string) (string, error)

	// StashShowText returns the raw unified diff for a stash entry (git only).
	StashShowText(input *types.StashShowInput, cwd string) (string, error)

	// RepoRoot returns the absolute root path of the repository.
	RepoRoot(input types.CLIInput, cwd string) (string, error)

	// UntrackedFiles returns repo-root-relative paths of untracked files.
	// Only applicable to git; jj always returns an empty slice.
	UntrackedFiles(input *types.VCSInput, repoRoot, cwd string) ([]string, error)

	// UntrackedFileDiffText returns the synthetic new-file diff for one untracked path.
	UntrackedFileDiffText(input *types.VCSInput, filePath, repoRoot, cwd string) (string, error)

	// DiffNumstat returns the numstat output for quickly determining file sizes.
	DiffNumstat(input *types.VCSInput, cwd string) (string, error)
}

// NumstatFile is one entry from a `git diff --numstat` or equivalent output.
type NumstatFile struct {
	Path      string
	Additions int
	Deletions int
}

// ParseNumstat parses `git diff --numstat -z` NUL-delimited output.
func ParseNumstat(text string) []NumstatFile {
	var files []NumstatFile
	parts := strings.Split(text, "\000")
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if p == "" {
			continue
		}
		fields := strings.SplitN(p, "\t", 3)
		if len(fields) < 3 {
			continue
		}
		adds, err1 := strconv.Atoi(strings.TrimSpace(fields[0]))
		dels, err2 := strconv.Atoi(strings.TrimSpace(fields[1]))
		if err1 != nil || err2 != nil {
			// Binary files show "-" counts; skip for size checks.
			continue
		}
		path := fields[2]
		// Rename entry: path is empty; next two NUL tokens are old/new paths.
		if path == "" {
			i += 2
			if i < len(parts) {
				path = parts[i]
			}
		}
		if path != "" {
			files = append(files, NumstatFile{Path: path, Additions: adds, Deletions: dels})
		}
	}
	return files
}

// CmdLabel returns a user-readable label for the given CLI input.
func CmdLabel(input types.CLIInput) string {
	switch v := input.(type) {
	case *types.VCSInput:
		if v.Staged {
			return "unk diff --staged"
		}
		if v.Range != nil {
			return "unk diff " + *v.Range
		}
		return "unk diff"
	case *types.ShowInput:
		if v.Ref != nil {
			return "unk show " + *v.Ref
		}
		return "unk show"
	case *types.StashShowInput:
		if v.Ref != nil {
			return "unk stash show " + *v.Ref
		}
		return "unk stash show"
	}
	return "unk"
}
