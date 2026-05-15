// Package loader normalises all CLI input sources into a Changeset.
package loader

import (
	"fmt"
	"os"
	"path/filepath"

	agentpkg "github.com/kpango/unk/internal/agent"
	"github.com/kpango/unk/internal/config"
	"github.com/kpango/unk/internal/types"
	"github.com/kpango/unk/internal/vcs"
	"github.com/kpango/unk/internal/vcs/git"
	"github.com/kpango/unk/internal/vcs/jj"
)

const (
	largeDiffFileMaxLines = 20_000
	largeDiffFileMaxBytes = 1_000_000
)

// Options is the working-directory path passed to loader calls.
// Use loader.Options(cwd) to construct.
type Options string

// Load is the main entry point: resolves config, loads the diff,
// and returns the full Bootstrap ready for the TUI.
// Load resolves config, loads the diff, and returns the full Bootstrap.
// Equivalent to LoadConfig followed by LoadChangeset.
func Load(input types.CLIInput, opts Options) (*types.Bootstrap, error) {
	bs, err := LoadConfig(input, opts)
	if err != nil {
		return nil, err
	}
	cwd := string(opts)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	if err := LoadChangeset(bs, cwd); err != nil {
		return nil, err
	}
	return bs, nil
}

// LoadConfig resolves config and display options without running git or reading
// patch data. Returns a Bootstrap with an empty Changeset but all display
// preferences (theme, mode, line-numbers, etc.) already populated from the
// config files and CLI flags. This is fast (< 1 ms) and safe to call before
// the TUI starts; the expensive data load is deferred to LoadChangeset.
func LoadConfig(input types.CLIInput, opts Options) (*types.Bootstrap, error) {
	cwd := string(opts)
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	cfgResult, err := config.Resolve(input, cwd)
	if err != nil {
		return nil, err
	}
	input = cfgResult.Input
	opts2 := types.OptionsOf(input)

	bootstrap := &types.Bootstrap{
		Input:       input,
		InitialMode: types.LayoutModeAuto,
	}
	if opts2.Mode != nil {
		bootstrap.InitialMode = *opts2.Mode
	}
	bootstrap.InitialTheme = opts2.Theme
	bootstrap.InitialKeymap = opts2.Keymap
	bootstrap.InitialShowLineNumbers = opts2.LineNumbers
	bootstrap.InitialWrapLines = opts2.WrapLines
	bootstrap.InitialShowUnkHeaders = opts2.UnkHeaders
	bootstrap.InitialShowAgentNotes = opts2.AgentNotes
	bootstrap.KeyBindingOverrides = cfgResult.KeyBindingOverrides
	bootstrap.CustomPalettes = cfgResult.CustomPalettes

	return bootstrap, nil
}

// LoadChangeset populates bs.Changeset and bs.RepoRoot using bs.Input, which
// must already be config-resolved (as returned by LoadConfig). Skips the
// config-resolution step that Load performs, so there is no double-layering.
// This is the slow part: it runs git (or reads the patch file) to build the diff.
func LoadChangeset(bs *types.Bootstrap, cwd string) error {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	input := bs.Input
	opts2 := types.OptionsOf(input)

	var (
		agentCtx *types.AgentContext
		err      error
	)
	if opts2.AgentContext != nil && *opts2.AgentContext != "" {
		agentPath := *opts2.AgentContext
		if agentPath != "-" && !filepath.IsAbs(agentPath) {
			agentPath = filepath.Join(cwd, agentPath)
		}
		agentCtx, err = agentpkg.Load(agentPath)
		if err != nil {
			return err
		}
	}

	var changeset *types.Changeset
	switch v := input.(type) {
	case *types.VCSInput:
		changeset, err = loadVCSChangeset(v, agentCtx, cwd)
	case *types.ShowInput:
		changeset, err = loadShowChangeset(v, agentCtx, cwd)
	case *types.StashShowInput:
		changeset, err = loadStashShowChangeset(v, agentCtx, cwd)
	case *types.FileInput:
		changeset, err = loadFileChangeset(v, agentCtx, cwd)
	case *types.PatchInput:
		changeset, err = loadPatchChangeset(v, agentCtx)
	case *types.DiffToolInput:
		changeset, err = loadDiffToolChangeset(v, agentCtx, cwd)
	default:
		return fmt.Errorf("unsupported input kind: %s", input.Kind())
	}
	if err != nil {
		return err
	}

	changeset.Files = agentpkg.OrderFiles(changeset.Files, agentCtx)

	switch input.(type) {
	case *types.VCSInput, *types.ShowInput, *types.StashShowInput:
		if client, clientErr := makeVCSClient(types.OptionsOf(input)); clientErr == nil {
			bs.RepoRoot, _ = client.RepoRoot(input, cwd)
		}
	}

	bs.Changeset = *changeset
	return nil
}

// makeVCSClient is a shared helper that constructs the appropriate VCS client.
func makeVCSClient(opts types.CommonOptions) (vcs.Client, error) {
	if opts.VCS != nil && *opts.VCS == types.VCSModeJJ {
		return jj.New(), nil
	}
	return git.New(), nil
}

// CanReloadInput reports whether the input source can be reloaded for watch mode.
// Stdin-backed patch inputs (no real file path) cannot be re-read.
func CanReloadInput(input types.CLIInput) bool {
	p, ok := input.(*types.PatchInput)
	if !ok {
		return true
	}
	// Text means the content was already consumed from stdin — cannot reload.
	if p.Text != nil {
		return false
	}
	// File == nil or "-" means stdin — cannot reload.
	if p.File == nil || *p.File == "-" {
		return false
	}
	return true
}
