package loader

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/kpango/unk/internal/diff"
	"github.com/kpango/unk/internal/types"
)

func loadFileChangeset(input *types.FileInput, agentCtx *types.AgentContext, cwd string) (*types.Changeset, error) {
	leftPath := resolvePath(cwd, input.Left)
	rightPath := resolvePath(cwd, input.Right)

	leftData, err := os.ReadFile(leftPath)
	if err != nil {
		return nil, err
	}
	rightData, err := os.ReadFile(rightPath)
	if err != nil {
		return nil, err
	}

	leftBase := filepath.Base(input.Left)
	rightBase := filepath.Base(input.Right)

	if diff.IsProbablyBinary(leftData) || diff.IsProbablyBinary(rightData) {
		cs := emptyChangeset(fmt.Sprintf("pair:%s", rightBase))
		placeholder := buildBinaryPlaceholder(rightBase, cs.ID, 0)
		placeholder.PreviousPath = &leftBase
		cs.Files = []types.DiffFile{placeholder}
		return cs, nil
	}

	// Use basenames in the diff headers so DiffFile.Path is display-friendly, not absolute.
	patchText := createTwoFilesPatch(leftBase, rightBase, string(leftData), string(rightData))
	title := fmt.Sprintf("%s ↔ %s", leftBase, rightBase)
	return normalizePatchChangeset(patchText, title, "file compare", agentCtx), nil
}

func loadDiffToolChangeset(input *types.DiffToolInput, agentCtx *types.AgentContext, cwd string) (*types.Changeset, error) {
	leftData, err := os.ReadFile(resolvePath(cwd, input.Left))
	if err != nil {
		return nil, err
	}
	rightData, err := os.ReadFile(resolvePath(cwd, input.Right))
	if err != nil {
		return nil, err
	}

	// Use display path in headers so DiffFile.Path is display-friendly, not absolute.
	displayPath := filepath.Base(input.Right)
	if input.Path != nil {
		displayPath = *input.Path
	}

	if diff.IsProbablyBinary(leftData) || diff.IsProbablyBinary(rightData) {
		cs := emptyChangeset(fmt.Sprintf("pair:%s", displayPath))
		placeholder := buildBinaryPlaceholder(displayPath, cs.ID, 0)
		leftBase := filepath.Base(input.Left)
		placeholder.PreviousPath = &leftBase
		cs.Files = []types.DiffFile{placeholder}
		return cs, nil
	}

	leftDisplay := filepath.Base(input.Left)
	patchText := createTwoFilesPatch(leftDisplay, displayPath, string(leftData), string(rightData))
	title := fmt.Sprintf("git difftool: %s", displayPath)
	return normalizePatchChangeset(patchText, title, "git difftool", agentCtx), nil
}

func loadPatchChangeset(input *types.PatchInput, agentCtx *types.AgentContext) (*types.Changeset, error) {
	var patchText string

	switch {
	case input.Text != nil:
		patchText = *input.Text
	case input.File != nil:
		path := *input.File
		var data []byte
		var err error
		if path == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(path)
		}
		if err != nil {
			return nil, err
		}
		patchText = string(data)
	default:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		patchText = string(data)
	}
	label := "unk patch"
	if input.File != nil && *input.File != "-" {
		label = "Patch review: " + filepath.Base(*input.File)
	}
	return normalizePatchChangeset(patchText, label, "unk patch", agentCtx), nil
}

// createTwoFilesPatch generates a proper contextual unified diff between two file contents.
func createTwoFilesPatch(leftPath, rightPath, leftContent, rightContent string) string {
	ud := difflib.UnifiedDiff{
		A:        difflib.SplitLines(leftContent),
		B:        difflib.SplitLines(rightContent),
		FromFile: "a/" + leftPath,
		ToFile:   "b/" + rightPath,
		Context:  3,
	}
	body, err := difflib.GetUnifiedDiffString(ud)
	if err != nil || body == "" {
		return ""
	}
	// Prepend a git-style diff --git header so the parser treats it correctly.
	return fmt.Sprintf("diff --git a/%s b/%s\n%s", leftPath, rightPath, body)
}

func resolvePath(cwd, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(cwd, p)
}
