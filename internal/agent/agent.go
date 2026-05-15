// Package agent loads and queries the optional AI-authored review sidecar.
package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/kpango/unk/internal/types"
)

// Load reads an AgentContext from a file path or from stdin when path is "-".
// Returns nil, nil when path is empty.
func Load(path string) (*types.AgentContext, error) {
	if path == "" {
		return nil, nil
	}

	var data []byte
	var err error

	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("reading agent context: %w", err)
	}

	var ctx types.AgentContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("parsing agent context JSON: %w", err)
	}
	return &ctx, nil
}

// FindFileContext returns the AgentFileContext for currentPath (or previousPath
// on a rename).
func FindFileContext(ctx *types.AgentContext, currentPath, previousPath string) *types.AgentFileContext {
	if ctx == nil {
		return nil
	}
	if i := slices.IndexFunc(ctx.Files, func(f types.AgentFileContext) bool { return f.Path == currentPath }); i >= 0 {
		return &ctx.Files[i]
	}
	if previousPath != "" {
		if i := slices.IndexFunc(ctx.Files, func(f types.AgentFileContext) bool { return f.Path == previousPath }); i >= 0 {
			return &ctx.Files[i]
		}
	}
	return nil
}

// OrderFiles returns files sorted so that any file mentioned in ctx.Files appears
// first (in sidecar order), with remaining files appended in their original order.
func OrderFiles(files []types.DiffFile, ctx *types.AgentContext) []types.DiffFile {
	if ctx == nil || len(ctx.Files) == 0 {
		return files
	}

	// Build position map from sidecar — indexed by both current path and previous path
	// to handle renamed files whose sidecar entry uses the old name.
	sidecarPos := make(map[string]int, len(ctx.Files)*2)
	for i, f := range ctx.Files {
		sidecarPos[f.Path] = i
	}

	// ranked uses nil pointer as "not yet placed" sentinel, eliminating a parallel
	// bool slice. A non-nil *DiffFile means the sidecar assigned this position.
	ranked := make([]*types.DiffFile, len(ctx.Files))
	var unranked []types.DiffFile

	for _, f := range files {
		pos, ok := sidecarPos[f.Path]
		if !ok && f.PreviousPath != nil {
			pos, ok = sidecarPos[*f.PreviousPath]
		}
		if ok {
			fp := f
			ranked[pos] = &fp
		} else {
			unranked = append(unranked, f)
		}
	}

	ordered := make([]types.DiffFile, 0, len(files))
	for _, fp := range ranked {
		if fp != nil {
			ordered = append(ordered, *fp)
		}
	}
	ordered = append(ordered, unranked...)
	return ordered
}
