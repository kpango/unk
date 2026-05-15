// Package git implements the unk VCS interface for Git repositories using go-git.
package git

import (
	"strings"

	gogit "github.com/go-git/go-git/v5"

	"github.com/kpango/unk/internal/syncmap"
	"github.com/kpango/unk/internal/types"
	"github.com/kpango/unk/internal/vcs"
)

// SubjectReader is an optional extension of vcs.Client for backends that can
// retrieve a commit's one-line subject. Test with a type assertion.
type SubjectReader interface {
	CommitSubject(ref, cwd string) string
}

// Option is a functional option for configuring a git Client.
type Option func(*client)

// client is the Git VCS backend backed by go-git (no subprocess required).
type client struct {
	repoCache syncmap.Map[string, *gogit.Repository]
}

// New creates a Git client implementing vcs.Client (and SubjectReader).
func New(opts ...Option) vcs.Client {
	c := &client{repoCache: syncmap.New[string, *gogit.Repository]()}
	for _, o := range opts {
		o(c)
	}
	return c
}

// openRepo opens (or returns a cached) repository containing path.
func (c *client) openRepo(path string) (*gogit.Repository, error) {
	if repo, ok := c.repoCache.Load(path); ok {
		return repo, nil
	}
	repo, err := gogit.PlainOpenWithOptions(path, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, err
	}
	c.repoCache.Store(path, repo)
	return repo, nil
}

// withRepo opens the repository for cwd and wraps any open error with a user message.
func (c *client) withRepo(input types.CLIInput, cwd string) (*gogit.Repository, error) {
	repo, err := c.openRepo(cwd)
	if err != nil {
		return nil, c.wrapOpenErr(input, err)
	}
	return repo, nil
}

// RepoRoot implements vcs.Client.
func (c *client) RepoRoot(input types.CLIInput, cwd string) (string, error) {
	repo, err := c.withRepo(input, cwd)
	if err != nil {
		return "", err
	}
	return wtRoot(repo)
}

// CommitSubject returns the one-line subject of the given ref.
func (c *client) CommitSubject(ref, cwd string) string {
	repo, err := c.openRepo(cwd)
	if err != nil {
		return ""
	}
	hash, err := resolveRev(repo, ref)
	if err != nil {
		return ""
	}
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return ""
	}
	return strings.SplitN(strings.TrimSpace(commit.Message), "\n", 2)[0]
}
