package git

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kpango/unk/internal/types"
)

// ShowText implements vcs.Client.
func (c *client) ShowText(input *types.ShowInput, cwd string) (string, error) {
	repo, err := c.withRepo(input, cwd)
	if err != nil {
		return "", err
	}
	ctx := context.Background()

	ref := "HEAD"
	if input.Ref != nil {
		ref = *input.Ref
	}
	hash, err := resolveRev(repo, ref)
	if err != nil {
		return "", c.invalidRevErr(input)
	}
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return "", c.invalidRevErr(input)
	}
	newTree, err := commit.Tree()
	if err != nil {
		return "", err
	}
	var oldTree *object.Tree
	if commit.NumParents() > 0 {
		if parent, pErr := commit.Parent(0); pErr == nil {
			oldTree, _ = parent.Tree()
		}
	}
	return encodeTreePatch(ctx, oldTree, newTree, input.Pathspecs)
}

// StashShowText implements vcs.Client.
func (c *client) StashShowText(input *types.StashShowInput, cwd string) (string, error) {
	repo, err := c.withRepo(input, cwd)
	if err != nil {
		return "", err
	}
	ctx := context.Background()

	// Stash entries live as commits under refs/stash (first-parent chain).
	stashIndex := 0
	if input.Ref != nil {
		var n int
		if _, scanErr := fmt.Sscanf(*input.Ref, "stash@{%d}", &n); scanErr == nil {
			stashIndex = n
		}
	}

	ref, err := repo.Reference(plumbing.ReferenceName("refs/stash"), true)
	if err != nil {
		return "", c.missingStashErr(input)
	}

	hash := ref.Hash()
	for i := 0; i < stashIndex; i++ {
		commit, cErr := repo.CommitObject(hash)
		if cErr != nil || commit.NumParents() == 0 {
			return "", c.missingStashErr(input)
		}
		parent, pErr := commit.Parent(0)
		if pErr != nil {
			return "", c.missingStashErr(input)
		}
		hash = parent.Hash
	}

	stashCommit, err := repo.CommitObject(hash)
	if err != nil || stashCommit.NumParents() == 0 {
		return "", c.missingStashErr(input)
	}
	baseCommit, err := stashCommit.Parent(0)
	if err != nil {
		return "", c.missingStashErr(input)
	}
	baseTree, err := baseCommit.Tree()
	if err != nil {
		return "", err
	}
	stashTree, err := stashCommit.Tree()
	if err != nil {
		return "", err
	}
	return encodeTreePatch(ctx, baseTree, stashTree, nil)
}
