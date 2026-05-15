package git

import (
	"os"
	"path/filepath"
	"strings"

	gogit "github.com/go-git/go-git/v5"

	"github.com/kpango/unk/internal/types"
)

// UntrackedFiles implements vcs.Client.
func (c *client) UntrackedFiles(input *types.VCSInput, repoRoot, cwd string) ([]string, error) {
	if input.Options.ExcludeUntracked != nil && *input.Options.ExcludeUntracked {
		return nil, nil
	}
	if input.Staged || input.Range != nil {
		return nil, nil
	}
	repo, err := c.withRepo(input, cwd)
	if err != nil {
		return nil, err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, err
	}
	status, err := wt.Status()
	if err != nil {
		return nil, err
	}
	var paths []string
	for path, fs := range status {
		if fs.Worktree == gogit.Untracked && fs.Staging == gogit.Untracked {
			if isReviewableUntrackedPath(repoRoot, path) {
				paths = append(paths, path)
			}
		}
	}
	return paths, nil
}

// UntrackedFileDiffText implements vcs.Client.
func (c *client) UntrackedFileDiffText(input *types.VCSInput, filePath, repoRoot, cwd string) (string, error) {
	absPath := filepath.Join(repoRoot, filePath)
	newData, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	return formatNewFileDiff(filePath, string(newData)), nil
}

// isReviewableUntrackedPath returns false for .unk/ entries and directories.
func isReviewableUntrackedPath(repoRoot, filePath string) bool {
	if strings.HasPrefix(filePath, ".unk/") || filePath == ".unk" {
		return false
	}
	abs := filepath.Join(repoRoot, filePath)
	info, err := os.Lstat(abs)
	if err != nil {
		return true
	}
	if info.IsDir() {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := os.Stat(abs)
		if err != nil {
			return true
		}
		return !resolved.IsDir()
	}
	return true
}
