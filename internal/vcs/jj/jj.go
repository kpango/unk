// Package jj implements the unk VCS interface for Jujutsu repositories.
package jj

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	unkerr "github.com/kpango/unk/internal/errors"
	"github.com/kpango/unk/internal/types"
	"github.com/kpango/unk/internal/vcs"
)

// Option is a functional option for configuring a jj client.
type Option func(*client)

// WithExe overrides the jj executable path (default: "jj").
func WithExe(exe string) Option {
	return func(c *client) { c.exePath = exe }
}

// client is the Jujutsu VCS backend.
type client struct {
	exePath string
}

// New creates a JJ client using the system jj executable.
func New(opts ...Option) vcs.Client {
	c := &client{exePath: "jj"}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *client) exe() string {
	if c.exePath != "" {
		return c.exePath
	}
	return "jj"
}

// run executes one jj command and returns (stdout, exitCode, stderr).
func (c *client) run(cwd string, args ...string) (string, int, string, error) {
	fullArgs := append([]string{"--no-pager", "--color", "never"}, args...)
	cmd := exec.Command(c.exe(), fullArgs...)
	cmd.Dir = cwd

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if ee, ok := err.(*exec.ExitError); ok {
		exitCode = ee.ExitCode()
		err = nil
	} else if err != nil {
		return "", -1, "", err
	}
	return stdout.String(), exitCode, stderr.String(), nil
}

// DiffText implements vcs.Client.
func (c *client) DiffText(input *types.VCSInput, cwd string) (string, error) {
	if input.Staged {
		return "", createStagedError(input)
	}
	args := buildDiffArgs(input)
	stdout, code, stderr, err := c.run(cwd, args...)
	if err != nil {
		return "", c.translateSpawnErr(input, err)
	}
	if code != 0 {
		return "", c.translateExitErr(input, stderr)
	}
	return stdout, nil
}

// ShowText implements vcs.Client.
func (c *client) ShowText(input *types.ShowInput, cwd string) (string, error) {
	args := buildShowArgs(input)
	stdout, code, stderr, err := c.run(cwd, args...)
	if err != nil {
		return "", c.translateSpawnErr(input, err)
	}
	if code != 0 {
		return "", c.translateExitErr(input, stderr)
	}
	return stdout, nil
}

// StashShowText is not supported by jj.
func (c *client) StashShowText(_ *types.StashShowInput, _ string) (string, error) {
	return "", unkerr.NewUserError("unk stash show is only supported in Git mode.",
		unkerr.WithDetails("Remove --staged or set vcs = \"git\" in Unk config."))
}

// RepoRoot implements vcs.Client.
func (c *client) RepoRoot(input types.CLIInput, cwd string) (string, error) {
	stdout, code, stderr, err := c.run(cwd, "root")
	if err != nil {
		return "", c.translateSpawnErr(input, err)
	}
	if code != 0 {
		return "", c.translateExitErr(input, stderr)
	}
	return strings.TrimSpace(stdout), nil
}

// DiffNumstat implements vcs.Client using `jj diff --stat`.
// The output is converted to tab-separated numstat lines for vcs.ParseNumstat.
func (c *client) DiffNumstat(input *types.VCSInput, cwd string) (string, error) {
	if input.Staged {
		return "", nil // JJ has no staging area
	}
	args := []string{"diff", "--stat"}
	if input.Range != nil {
		args = append(args, "-r", *input.Range)
	}
	if len(input.Pathspecs) > 0 {
		args = append(args, "--")
		args = append(args, input.Pathspecs...)
	}
	stdout, code, _, err := c.run(cwd, args...)
	if err != nil || code != 0 {
		return "", nil // best-effort; skip large-file detection on failure
	}
	return parseJJStatToNumstat(stdout), nil
}

// UntrackedFiles always returns nil for jj — jj has no concept of untracked files.
func (c *client) UntrackedFiles(_ *types.VCSInput, _, _ string) ([]string, error) {
	return nil, nil
}

// UntrackedFileDiffText always returns empty for jj.
func (c *client) UntrackedFileDiffText(_ *types.VCSInput, _, _, _ string) (string, error) {
	return "", nil
}

// createStagedError returns the canonical error for jj + --staged.
func createStagedError(input *types.VCSInput) error {
	label := vcs.CmdLabel(input)
	return unkerr.NewUserError(
		fmt.Sprintf("`%s` requires Git VCS mode because Jujutsu has no staging area.", label),
		unkerr.WithDetails("Remove `--staged`, or set `vcs = \"git\"` in Unk config."),
	)
}

// appendPathspecs appends "-- <pathspecs>" to args when pathspecs is non-empty.
func appendPathspecs(args, pathspecs []string) []string {
	if len(pathspecs) == 0 {
		return args
	}
	return append(append(args, "--"), pathspecs...)
}

func buildDiffArgs(input *types.VCSInput) []string {
	args := []string{"diff", "--git"}
	if input.Range != nil {
		args = append(args, "-r", *input.Range)
	}
	return appendPathspecs(args, input.Pathspecs)
}

func buildShowArgs(input *types.ShowInput) []string {
	ref := "@"
	if input.Ref != nil {
		ref = *input.Ref
	}
	return appendPathspecs([]string{"diff", "--git", "-r", ref}, input.Pathspecs)
}

func (c *client) translateSpawnErr(input types.CLIInput, err error) error {
	msg := err.Error()
	if strings.Contains(msg, "executable file not found") || strings.Contains(msg, "no such file") {
		return unkerr.NewUserError(
			fmt.Sprintf("Jujutsu is required for `%s` when `vcs = \"jj\"`, but `%s` was not found in PATH.", vcs.CmdLabel(input), c.exe()),
			unkerr.WithDetails("Install Jujutsu or set `vcs = \"git\"` in Unk config, then try again."),
		)
	}
	return err
}

func (c *client) translateExitErr(input types.CLIInput, stderr string) error {
	if isMissingRepoMsg(stderr) {
		return unkerr.NewUserError(
			fmt.Sprintf("`%s` must be run inside a Jujutsu repository when `vcs = \"jj\"`.", vcs.CmdLabel(input)),
			unkerr.WithDetails("Run the command from a Jujutsu checkout, or set `vcs = \"git\"` in Unk config."),
		)
	}
	if isInvalidRevsetMsg(stderr) {
		revset := revsetLabel(input)
		return unkerr.NewUserError(
			fmt.Sprintf("`%s` could not resolve Jujutsu revset `%s`.", vcs.CmdLabel(input), revset),
			unkerr.WithDetails("Check the revset and try again."),
		)
	}
	return unkerr.NewUserError(fmt.Sprintf("`%s` failed.", vcs.CmdLabel(input)),
		unkerr.WithDetails(firstErrLine(stderr)))
}

func isMissingRepoMsg(s string) bool {
	return strings.Contains(s, "There is no jj repo in") || strings.Contains(s, "not in a workspace")
}

func isInvalidRevsetMsg(s string) bool {
	for _, frag := range []string{
		"Failed to parse revset", "Revision not found", "No such revision",
		"doesn't exist", "is ambiguous", "Revset expression resolved to no revisions",
	} {
		if strings.Contains(s, frag) {
			return true
		}
	}
	return false
}

func revsetLabel(input types.CLIInput) string {
	switch v := input.(type) {
	case *types.VCSInput:
		if v.Range != nil {
			return *v.Range
		}
	case *types.ShowInput:
		if v.Ref != nil {
			return *v.Ref
		}
		return "@"
	}
	return ""
}

// parseJJStatToNumstat converts `jj diff --stat` output into tab-delimited numstat lines.
func parseJJStatToNumstat(stat string) string {
	var sb strings.Builder
	for line := range strings.SplitSeq(stat, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "0 files") || strings.HasPrefix(line, "1 file") ||
			(strings.Contains(line, "file") && strings.Contains(line, "changed")) {
			continue
		}
		idx := strings.LastIndex(line, "|")
		if idx < 0 {
			continue
		}
		pathPart := strings.TrimSpace(line[:idx])
		rest := strings.TrimSpace(line[idx+1:])
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		total, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		sb.WriteString(strconv.Itoa(total))
		sb.WriteByte('\t')
		sb.WriteString("0")
		sb.WriteByte('\t')
		sb.WriteString(pathPart)
		sb.WriteByte('\000')
	}
	return sb.String()
}

func firstErrLine(stderr string) string {
	for line := range strings.SplitSeq(stderr, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return strings.TrimPrefix(line, "error: ")
		}
	}
	return "Jujutsu command failed."
}
