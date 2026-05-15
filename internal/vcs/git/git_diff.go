package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kpango/unk/internal/types"
)

// DiffText implements vcs.Client.
func (c *client) DiffText(input *types.VCSInput, cwd string) (string, error) {
	repo, err := c.withRepo(input, cwd)
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	switch {
	case input.Staged:
		return c.diffStaged(repo, input)
	case input.Range != nil:
		return c.diffRange(ctx, repo, input)
	default:
		return c.diffWorkingTree(repo, input)
	}
}

// DiffNumstat implements vcs.Client.
func (c *client) DiffNumstat(input *types.VCSInput, cwd string) (string, error) {
	patch, err := c.DiffText(input, cwd)
	if err != nil {
		return "", err
	}
	return patchToNumstat(patch), nil
}

// diffRange diffs two commits resolved from a range expression (e.g. "HEAD~2..HEAD").
func (c *client) diffRange(ctx context.Context, repo *gogit.Repository, input *types.VCSInput) (string, error) {
	rangeStr := *input.Range

	var oldHash, newHash plumbing.Hash
	sep := rangeStr
	if idx := strings.Index(rangeStr, "..."); idx >= 0 {
		parts := strings.SplitN(rangeStr, "...", 2)
		oh, err := resolveRev(repo, parts[0])
		if err != nil {
			return "", c.invalidRevErr(input)
		}
		nh, err := resolveRev(repo, parts[1])
		if err != nil {
			return "", c.invalidRevErr(input)
		}
		oldHash, newHash = oh, nh
	} else if idx = strings.Index(sep, ".."); idx >= 0 {
		parts := strings.SplitN(rangeStr, "..", 2)
		oh, err := resolveRev(repo, parts[0])
		if err != nil {
			return "", c.invalidRevErr(input)
		}
		nh, err := resolveRev(repo, parts[1])
		if err != nil {
			return "", c.invalidRevErr(input)
		}
		oldHash, newHash = oh, nh
	} else {
		// Single ref: compare against its first parent.
		nh, err := resolveRev(repo, rangeStr)
		if err != nil {
			return "", c.invalidRevErr(input)
		}
		newHash = nh
		commit, cErr := repo.CommitObject(nh)
		if cErr != nil {
			return "", c.invalidRevErr(input)
		}
		if commit.NumParents() > 0 {
			parent, pErr := commit.Parent(0)
			if pErr != nil {
				return "", pErr
			}
			oldHash = parent.Hash
		}
	}

	oldCommit, err := repo.CommitObject(oldHash)
	if err != nil {
		return "", c.invalidRevErr(input)
	}
	newCommit, err := repo.CommitObject(newHash)
	if err != nil {
		return "", c.invalidRevErr(input)
	}
	oldTree, err := oldCommit.Tree()
	if err != nil {
		return "", err
	}
	newTree, err := newCommit.Tree()
	if err != nil {
		return "", err
	}
	return encodeTreePatch(ctx, oldTree, newTree, input.Pathspecs)
}

// diffStaged diffs HEAD tree vs the index (staging area).
// It reads blob content directly from the object store — no temporary objects written.
func (c *client) diffStaged(repo *gogit.Repository, input *types.VCSInput) (string, error) {
	idx, err := repo.Storer.Index()
	if err != nil {
		return "", err
	}

	// Build content maps: path → file content string.
	headContent := map[string]string{}
	headTree, _ := headTreeOf(repo)
	if headTree != nil {
		iter := headTree.Files()
		if iter != nil {
			_ = iter.ForEach(func(f *object.File) error {
				s, cErr := f.Contents()
				if cErr == nil {
					headContent[f.Name] = s
				}
				return nil
			})
		}
	}

	indexContent := make(map[string]string, len(idx.Entries))
	for _, e := range idx.Entries {
		s, rErr := readBlob(repo, e.Hash)
		if rErr == nil {
			indexContent[e.Name] = s
		}
	}

	return buildPatchFromContent(headContent, indexContent, input.Pathspecs), nil
}

// diffWorkingTree diffs HEAD tree vs actual files on disk.
// Reads HEAD content from blob store and disk content directly — no blob writes.
func (c *client) diffWorkingTree(repo *gogit.Repository, input *types.VCSInput) (string, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	status, err := wt.Status()
	if err != nil {
		return "", err
	}
	root, err := wtRoot(repo)
	if err != nil {
		return "", err
	}

	// Collect HEAD content only for files that have working-tree changes.
	changedPaths := make(map[string]gogit.StatusCode, len(status))
	for path, fs := range status {
		if fs.Worktree != gogit.Unmodified && fs.Worktree != gogit.Untracked {
			changedPaths[path] = fs.Worktree
		}
	}

	headContent := make(map[string]string, len(changedPaths))
	headTree, _ := headTreeOf(repo)
	if headTree != nil {
		iter := headTree.Files()
		if iter != nil {
			_ = iter.ForEach(func(f *object.File) error {
				if _, changed := changedPaths[f.Name]; changed {
					s, cErr := f.Contents()
					if cErr == nil {
						headContent[f.Name] = s
					}
				}
				return nil
			})
		}
	}

	diskContent := make(map[string]string, len(changedPaths))
	for path, wc := range changedPaths {
		if wc == gogit.Deleted {
			continue // absence means deletion in buildPatchFromContent
		}
		data, readErr := os.ReadFile(filepath.Join(root, path))
		if readErr == nil {
			diskContent[path] = string(data)
		}
	}

	return buildPatchFromContent(headContent, diskContent, input.Pathspecs), nil
}
