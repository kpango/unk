// Command unk is a terminal-first diff viewer for understanding coding-agent changesets.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	unkerr "github.com/kpango/unk/internal/errors"
	"github.com/kpango/unk/internal/pager"
	"github.com/kpango/unk/internal/runner"
	"github.com/kpango/unk/internal/types"
)

// Version is set at build time via -ldflags "-X main.Version=x.y.z".
var Version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := rootCmd().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, unkerr.FormatCLIError(err))
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "unk",
		Short:         "Desktop-inspired terminal diff viewer for agent-authored changesets.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(diffCmd(), showCmd(), stashCmd(), patchCmd(), pagerCmd(), diffToolCmd())
	return root
}

// -- Common flags --

type commonFlags struct {
	mode         string
	theme        string
	keymap       string
	agentContext string
	pager        bool
	watch        bool
}

func addCommonFlags(cmd *cobra.Command, cf *commonFlags) {
	cmd.Flags().StringVar(&cf.mode, "mode", "", "layout mode: auto, split, stack")
	cmd.Flags().StringVar(&cf.theme, "theme", "", "named theme override")
	cmd.Flags().StringVar(&cf.keymap, "keymap", "", "key bindings style: helix (default), vim, emacs")
	cmd.Flags().StringVar(&cf.agentContext, "agent-context", "", "JSON sidecar with agent rationale")
	cmd.Flags().BoolVar(&cf.pager, "pager", false, "use pager-style chrome and controls")
}

func addWatchFlag(cmd *cobra.Command, cf *commonFlags) {
	cmd.Flags().BoolVar(&cf.watch, "watch", false, "auto-reload when the current diff input changes")
}

func buildCommonOptions(cf *commonFlags, args []string) types.CommonOptions {
	opts := types.CommonOptions{}
	if cf.mode != "" {
		m := types.LayoutMode(cf.mode)
		opts.Mode = &m
	}
	if cf.theme != "" {
		opts.Theme = &cf.theme
	}
	if cf.keymap != "" {
		opts.Keymap = &cf.keymap
	}
	if cf.agentContext != "" {
		opts.AgentContext = &cf.agentContext
	}
	if cf.pager {
		t := true
		opts.Pager = &t
	}
	if cf.watch {
		t := true
		opts.Watch = &t
	}
	// Paired boolean flags (--flag / --no-flag) are resolved from raw args.
	opts.ExcludeUntracked = resolveBoolFlag(args, "--exclude-untracked", "--no-exclude-untracked")
	opts.LineNumbers = resolveBoolFlag(args, "--line-numbers", "--no-line-numbers")
	opts.WrapLines = resolveBoolFlag(args, "--wrap", "--no-wrap")
	opts.UnkHeaders = resolveBoolFlag(args, "--unk-headers", "--no-unk-headers")
	opts.AgentNotes = resolveBoolFlag(args, "--agent-notes", "--no-agent-notes")
	return opts
}

func resolveBoolFlag(args []string, on, off string) *bool {
	var result *bool
	for _, a := range args {
		if a == on {
			v := true
			result = &v
		} else if a == off {
			v := false
			result = &v
		}
	}
	return result
}

func addPairedBoolFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("exclude-untracked", false, "hide untracked files in working tree reviews")
	cmd.Flags().Bool("no-exclude-untracked", false, "show untracked files (default)")
	cmd.Flags().Bool("line-numbers", false, "show line numbers")
	cmd.Flags().Bool("no-line-numbers", false, "hide line numbers")
	cmd.Flags().Bool("wrap", false, "wrap long diff lines")
	cmd.Flags().Bool("no-wrap", false, "truncate long diff lines")
	cmd.Flags().Bool("unk-headers", false, "show unk metadata rows")
	cmd.Flags().Bool("no-unk-headers", false, "hide unk metadata rows")
	cmd.Flags().Bool("agent-notes", false, "show agent notes by default")
	cmd.Flags().Bool("no-agent-notes", false, "hide agent notes by default")
	_ = cmd.Flags().MarkHidden("no-exclude-untracked")
	_ = cmd.Flags().MarkHidden("no-line-numbers")
	_ = cmd.Flags().MarkHidden("no-wrap")
	_ = cmd.Flags().MarkHidden("no-unk-headers")
	_ = cmd.Flags().MarkHidden("no-agent-notes")
}

// -- diff command --

func diffCmd() *cobra.Command {
	var cf commonFlags
	var staged bool
	cmd := &cobra.Command{
		Use:   "diff [target] [-- pathspec...]",
		Short: "Review working-tree changes or compare against a target",
		RunE: func(cmd *cobra.Command, args []string) error {
			rawArgs := os.Args[1:]
			opts := buildCommonOptions(&cf, rawArgs)
			cmdArgs, pathspecs := splitPathspecs(args)

			var input types.CLIInput
			switch len(cmdArgs) {
			case 0:
				input = &types.VCSInput{Staged: staged, Pathspecs: pathspecs}
			case 1:
				r := cmdArgs[0]
				input = &types.VCSInput{Range: &r, Staged: staged, Pathspecs: pathspecs}
			case 2:
				left, right := cmdArgs[0], cmdArgs[1]
				if !staged && len(pathspecs) == 0 && fileExists(left) && fileExists(right) {
					input = &types.FileInput{Left: left, Right: right}
				} else {
					r := left
					input = &types.VCSInput{Range: &r, Staged: staged, Pathspecs: append([]string{right}, pathspecs...)}
				}
			default:
				r := cmdArgs[0]
				input = &types.VCSInput{Range: &r, Staged: staged, Pathspecs: append(cmdArgs[1:], pathspecs...)}
			}
			return runner.Run(cmd.Context(), input.SetOptions(opts), Version)
		},
	}
	addCommonFlags(cmd, &cf)
	addWatchFlag(cmd, &cf)
	addPairedBoolFlags(cmd)
	cmd.Flags().BoolVar(&staged, "staged", false, "review staged changes")
	cmd.Flags().BoolVar(&staged, "cached", false, "review staged changes (alias for --staged)")
	_ = cmd.Flags().MarkHidden("cached")
	return cmd
}

// -- show command --

func showCmd() *cobra.Command {
	var cf commonFlags
	cmd := &cobra.Command{
		Use:   "show [ref] [-- pathspec...]",
		Short: "Review the last commit or a given ref",
		RunE: func(cmd *cobra.Command, args []string) error {
			rawArgs := os.Args[1:]
			opts := buildCommonOptions(&cf, rawArgs)
			cmdArgs, pathspecs := splitPathspecs(args)
			var ref *string
			if len(cmdArgs) > 0 {
				r := cmdArgs[0]
				ref = &r
			}
			return runner.Run(cmd.Context(), (&types.ShowInput{Ref: ref, Pathspecs: pathspecs}).SetOptions(opts), Version)
		},
	}
	addCommonFlags(cmd, &cf)
	addWatchFlag(cmd, &cf)
	addPairedBoolFlags(cmd)
	return cmd
}

// -- stash command --

func stashCmd() *cobra.Command {
	stash := &cobra.Command{Use: "stash", Short: "Stash operations"}
	stash.AddCommand(stashShowCmd())
	return stash
}

func stashShowCmd() *cobra.Command {
	var cf commonFlags
	cmd := &cobra.Command{
		Use:   "show [ref]",
		Short: "Review a stash entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			rawArgs := os.Args[1:]
			opts := buildCommonOptions(&cf, rawArgs)
			var ref *string
			if len(args) > 0 {
				r := args[0]
				ref = &r
			}
			return runner.Run(cmd.Context(), (&types.StashShowInput{Ref: ref}).SetOptions(opts), Version)
		},
	}
	addCommonFlags(cmd, &cf)
	addWatchFlag(cmd, &cf)
	addPairedBoolFlags(cmd)
	return cmd
}

// -- patch command --

func patchCmd() *cobra.Command {
	var cf commonFlags
	cmd := &cobra.Command{
		Use:   "patch [file]",
		Short: "Review a patch file or stdin",
		RunE: func(cmd *cobra.Command, args []string) error {
			rawArgs := os.Args[1:]
			opts := buildCommonOptions(&cf, rawArgs)
			var file *string
			if len(args) > 0 {
				file = &args[0]
			}
			return runner.Run(cmd.Context(), (&types.PatchInput{File: file}).SetOptions(opts), Version)
		},
	}
	addCommonFlags(cmd, &cf)
	addWatchFlag(cmd, &cf)
	addPairedBoolFlags(cmd)
	return cmd
}

// -- pager command --

func pagerCmd() *cobra.Command {
	var cf commonFlags
	cf.pager = true
	cmd := &cobra.Command{
		Use:   "pager",
		Short: "General Git pager wrapper with diff detection",
		RunE: func(cmd *cobra.Command, args []string) error {
			rawArgs := os.Args[1:]
			opts := buildCommonOptions(&cf, rawArgs)
			t := true
			opts.Pager = &t

			// Passthrough when stdout is not a terminal (e.g. piped to grep).
			if fi, err := os.Stdout.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
				_, err := io.Copy(os.Stdout, os.Stdin)
				return err
			}
			// Passthrough when running in a dumb terminal.
			if os.Getenv("TERM") == "dumb" {
				_, err := io.Copy(os.Stdout, os.Stdin)
				return err
			}
			// Exit cleanly when stdin is a terminal — no piped data to page.
			if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
				fmt.Fprintln(os.Stderr, "unk pager: no input — pipe git output to this command or set it as core.pager")
				return nil
			}

			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			text := strings.TrimRight(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
			if text == "" {
				return nil
			}

			if pager.LooksLikePatch(text) {
				return runner.Run(cmd.Context(), (&types.PatchInput{Text: &text}).SetOptions(opts), Version)
			}
			return pager.RunPlainText(text)
		},
	}
	addCommonFlags(cmd, &cf)
	addWatchFlag(cmd, &cf)
	addPairedBoolFlags(cmd)
	return cmd
}

// -- difftool command --

func diffToolCmd() *cobra.Command {
	var cf commonFlags
	cmd := &cobra.Command{
		Use:   "difftool <left> <right> [path]",
		Short: "Review Git difftool file pairs",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawArgs := os.Args[1:]
			opts := buildCommonOptions(&cf, rawArgs)
			var path *string
			if len(args) > 2 {
				path = &args[2]
			}
			return runner.Run(cmd.Context(), (&types.DiffToolInput{Left: args[0], Right: args[1], Path: path}).SetOptions(opts), Version)
		},
	}
	addCommonFlags(cmd, &cf)
	addWatchFlag(cmd, &cf)
	addPairedBoolFlags(cmd)
	return cmd
}

// -- utility helpers --

func splitPathspecs(args []string) (cmdArgs []string, pathspecs []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
