package git

import (
	"fmt"

	gogit "github.com/go-git/go-git/v5"

	unkerr "github.com/kpango/unk/internal/errors"
	"github.com/kpango/unk/internal/types"
	"github.com/kpango/unk/internal/vcs"
)

func (c *client) wrapOpenErr(input types.CLIInput, err error) error {
	if err == gogit.ErrRepositoryNotExists {
		return c.missingRepoErr(input)
	}
	return err
}

func (c *client) missingRepoErr(input types.CLIInput) error {
	label := vcs.CmdLabel(input)
	if input.Kind() != "vcs" {
		return unkerr.NewUserError(
			fmt.Sprintf("`%s` must be run inside a Git repository.", label),
			unkerr.WithDetails("Run the command from a Git checkout."),
		)
	}
	return unkerr.NewUserError(
		fmt.Sprintf("`%s` must be run inside a Git repository.", label),
		unkerr.WithDetails(
			"Run the command from a Git checkout, or compare files directly instead:",
			"  unk diff <before-file> <after-file>",
			"  unk patch <file.patch>",
		),
	)
}

func (c *client) missingStashErr(input types.CLIInput) error {
	if s, ok := input.(*types.StashShowInput); ok && s.Ref != nil {
		return unkerr.NewUserError(
			fmt.Sprintf("`%s` could not resolve stash entry `%s`.", vcs.CmdLabel(input), *s.Ref),
			unkerr.WithDetails("List available stashes with `git stash list`, then try again."),
		)
	}
	return unkerr.NewUserError(
		"`unk stash show` could not find a stash entry to show.",
		unkerr.WithDetails("Create one with `git stash push`, or pass an explicit stash ref like `unk stash show stash@{0}`."),
	)
}

func (c *client) invalidRevErr(input types.CLIInput) error {
	switch v := input.(type) {
	case *types.VCSInput:
		if v.Range != nil {
			return unkerr.NewUserError(
				fmt.Sprintf("`%s` could not resolve Git revision or range `%s`.", vcs.CmdLabel(input), *v.Range),
				unkerr.WithDetails("Check the revision or range and try again."),
			)
		}
	case *types.ShowInput:
		ref := "HEAD"
		if v.Ref != nil {
			ref = *v.Ref
		}
		return unkerr.NewUserError(
			fmt.Sprintf("`%s` could not resolve Git ref `%s`.", vcs.CmdLabel(input), ref),
			unkerr.WithDetails("Check the ref name and try again."),
		)
	}
	return unkerr.NewUserError(fmt.Sprintf("`%s` failed.", vcs.CmdLabel(input)))
}
