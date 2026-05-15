package loader

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/kpango/unk/internal/concurrent"
	unkerr "github.com/kpango/unk/internal/errors"
	"github.com/kpango/unk/internal/types"
	"github.com/kpango/unk/internal/vcs"
	"github.com/kpango/unk/internal/vcs/git"
)

func loadVCSChangeset(input *types.VCSInput, agentCtx *types.AgentContext, cwd string) (*types.Changeset, error) {
	opts := types.OptionsOf(input)
	client, err := makeVCSClient(opts)
	if err != nil {
		return nil, err
	}

	repoRoot, err := client.RepoRoot(input, cwd)
	if err != nil {
		return nil, err
	}

	// Step 1: Run numstat and untracked file detection concurrently.
	var (
		numstatText    string
		untrackedPaths []string
		numstatErr     error
		untrackedErr   error
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		numstatText, numstatErr = client.DiffNumstat(input, cwd)
	}()
	go func() {
		defer wg.Done()
		untrackedPaths, untrackedErr = client.UntrackedFiles(input, repoRoot, cwd)
	}()
	wg.Wait()

	if numstatErr != nil {
		return nil, numstatErr
	}
	if untrackedErr != nil {
		return nil, untrackedErr
	}

	numstatFiles := vcs.ParseNumstat(numstatText)
	numstatByPath := make(map[string]vcs.NumstatFile, len(numstatFiles))
	for _, f := range numstatFiles {
		numstatByPath[f.Path] = f
	}
	var largeFiles []string
	for _, f := range numstatFiles {
		if f.Additions+f.Deletions > largeDiffFileMaxLines {
			largeFiles = append(largeFiles, f.Path)
		}
	}

	// Step 2: Fetch the full diff (needs largeFiles list from numstat).
	patchText, err := diffTextExcluding(client, input, largeFiles, cwd)
	if err != nil {
		return nil, err
	}

	changeset := normalizePatchChangeset(patchText, vcsTitle(input), vcsSourceLabel(repoRoot), agentCtx)

	// Add placeholder entries for oversized files.
	for _, path := range largeFiles {
		var s types.DiffStats
		if e, ok := numstatByPath[path]; ok {
			s = types.DiffStats{Additions: e.Additions, Deletions: e.Deletions}
		}
		changeset.Files = append(changeset.Files, buildTooLargeFile(path, changeset.ID, s))
	}

	// Step 3: Fetch untracked file diffs concurrently.
	type untrackedFile struct {
		tooLarge bool
		stats    types.DiffStats
		patch    string
	}
	fileResults := concurrent.Map(untrackedPaths, func(p string) (untrackedFile, error) {
		absPath := filepath.Join(repoRoot, p)
		if info, statErr := os.Lstat(absPath); statErr == nil && info.Size() > largeDiffFileMaxBytes {
			var lineCount int
			if data, readErr := os.ReadFile(absPath); readErr == nil {
				lineCount = strings.Count(string(data), "\n")
			}
			return untrackedFile{tooLarge: true, stats: types.DiffStats{Additions: lineCount}}, nil
		}
		patch, pErr := client.UntrackedFileDiffText(input, p, repoRoot, cwd)
		if pErr != nil {
			return untrackedFile{}, pErr
		}
		if strings.Count(patch, "\n") > largeDiffFileMaxLines {
			return untrackedFile{tooLarge: true}, nil
		}
		return untrackedFile{patch: patch}, nil
	})

	for i, res := range fileResults {
		if res.Err != nil {
			return nil, res.Err
		}
		r := res.Value
		p := untrackedPaths[i]
		if r.tooLarge {
			changeset.Files = append(changeset.Files, buildTooLargeFile(p, changeset.ID, r.stats))
		} else {
			f := buildUntrackedDiffFile(p, r.patch, changeset.ID, len(changeset.Files), agentCtx)
			changeset.Files = append(changeset.Files, f)
		}
	}

	return changeset, nil
}

func loadShowChangeset(input *types.ShowInput, agentCtx *types.AgentContext, cwd string) (*types.Changeset, error) {
	opts := types.OptionsOf(input)
	client, err := makeVCSClient(opts)
	if err != nil {
		return nil, err
	}
	patchText, err := client.ShowText(input, cwd)
	if err != nil {
		return nil, err
	}
	label := "unk show"
	if input.Ref != nil {
		label = "unk show " + *input.Ref
	}
	cs := normalizePatchChangeset(patchText, label, label, agentCtx)

	// Extract commit subject from git log when available.
	if sr, ok := client.(git.SubjectReader); ok {
		ref := "HEAD"
		if input.Ref != nil {
			ref = *input.Ref
		}
		if subj := sr.CommitSubject(ref, cwd); subj != "" {
			cs.Summary = &subj
		}
	}
	return cs, nil
}

func loadStashShowChangeset(input *types.StashShowInput, agentCtx *types.AgentContext, cwd string) (*types.Changeset, error) {
	opts := types.OptionsOf(input)
	if opts.VCS != nil && *opts.VCS == types.VCSModeJJ {
		return nil, unkerr.NewUserError("`unk stash show` is not supported in Jujutsu mode.",
			unkerr.WithDetails("Use `unk diff` instead, or set `vcs = \"git\"` in Unk config."))
	}
	client, err := makeVCSClient(opts)
	if err != nil {
		return nil, err
	}
	patchText, err := client.StashShowText(input, cwd)
	if err != nil {
		return nil, err
	}
	label := "unk stash show"
	if input.Ref != nil {
		label = "unk stash show " + *input.Ref
	}
	return normalizePatchChangeset(patchText, label, label, agentCtx), nil
}

func diffTextExcluding(client vcs.Client, input *types.VCSInput, excludePaths []string, cwd string) (string, error) {
	if len(excludePaths) == 0 {
		return client.DiffText(input, cwd)
	}
	// Build exclusion pathspecs using git's :(exclude) magic pathspec.
	negated := concurrent.Transform(excludePaths, func(p string) string { return ":(exclude)" + p })
	modified := *input
	modified.Pathspecs = slices.Concat(input.Pathspecs, negated)
	return client.DiffText(&modified, cwd)
}

func vcsTitle(input *types.VCSInput) string {
	if input.Staged {
		return "unk diff --staged"
	}
	if input.Range != nil {
		return "unk diff " + *input.Range
	}
	return "unk diff"
}

func vcsSourceLabel(repoRoot string) string {
	base := filepath.Base(repoRoot)
	if base == "" || base == "." {
		base = repoRoot
	}
	return base
}
