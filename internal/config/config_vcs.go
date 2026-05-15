package config

import (
	"os"
	"path/filepath"

	"github.com/kpango/unk/internal/types"
)

// FindRepoRoot walks from cwd upward looking for a .git or .jj directory.
func FindRepoRoot(cwd string) string {
	current, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		if _, err := os.Stat(filepath.Join(current, ".jj")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

// DetectVCSMode returns the VCS backend appropriate for repoRoot.
func DetectVCSMode(repoRoot string) types.VCSMode {
	if repoRoot != "" {
		if _, err := os.Stat(filepath.Join(repoRoot, ".jj")); err == nil {
			return types.VCSModeJJ
		}
	}
	return types.VCSModeGit
}
