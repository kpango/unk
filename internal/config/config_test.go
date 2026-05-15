package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kpango/unk/internal/types"
)

func TestFindRepoRoot(t *testing.T) {
	tmp := t.TempDir()

	// No VCS directory → returns empty string.
	if got := FindRepoRoot(tmp); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	// .git present at root → finds root.
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := FindRepoRoot(tmp); got != tmp {
		t.Errorf("expected %q, got %q", tmp, got)
	}

	// Works from subdirectory.
	sub := filepath.Join(tmp, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := FindRepoRoot(sub); got != tmp {
		t.Errorf("expected %q, got %q", tmp, got)
	}
}

func TestDetectVCSMode(t *testing.T) {
	tmp := t.TempDir()

	// Default to git when neither .git nor .jj exists.
	if m := DetectVCSMode(tmp); m != types.VCSModeGit {
		t.Errorf("expected git, got %s", m)
	}

	// .jj present → jj.
	if err := os.MkdirAll(filepath.Join(tmp, ".jj"), 0o755); err != nil {
		t.Fatal(err)
	}
	if m := DetectVCSMode(tmp); m != types.VCSModeJJ {
		t.Errorf("expected jj, got %s", m)
	}
}

func TestMergeOptions(t *testing.T) {
	trueVal := true
	falseVal := false
	light := types.ThemeModeLight
	_ = light

	modeAuto := types.LayoutModeAuto
	modeSplit := types.LayoutModeSplit
	git := types.VCSModeGit

	base := types.CommonOptions{
		Mode:        &modeAuto,
		VCS:         &git,
		LineNumbers: &trueVal,
		WrapLines:   &falseVal,
	}

	overrides := types.CommonOptions{
		Mode:        &modeSplit,
		LineNumbers: &falseVal,
	}

	result := mergeOptions(base, overrides)

	if *result.Mode != types.LayoutModeSplit {
		t.Errorf("expected split mode, got %s", *result.Mode)
	}
	if *result.VCS != types.VCSModeGit {
		t.Errorf("expected git vcs to pass through, got %s", *result.VCS)
	}
	if *result.LineNumbers {
		t.Errorf("expected lineNumbers overridden to false")
	}
	if *result.WrapLines {
		t.Errorf("expected wrapLines to remain false from base")
	}
}

func TestResolveGlobalConfigDir(t *testing.T) {
	t.Run("XDG_CONFIG_HOME set", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/custom/config")
		p := resolveGlobalConfigDir()
		if p != "/custom/config/unk" {
			t.Errorf("unexpected path: %q", p)
		}
	})

	t.Run("falls back to HOME/.config", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "/home/user")
		p := resolveGlobalConfigDir()
		if p != "/home/user/.config/unk" {
			t.Errorf("unexpected path: %q", p)
		}
	})
}

func TestTomlConfigResolution(t *testing.T) {
	tmp := t.TempDir()

	// Write a repo config with a mode override.
	repoUnkDir := filepath.Join(tmp, ".unk")
	if err := os.MkdirAll(repoUnkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(repoUnkDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(`mode = "split"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Plant .git so the repo root is detected.
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", "") // no global config
	t.Setenv("HOME", "")

	input := &types.VCSInput{}
	result, err := Resolve(input, tmp)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	opts := types.OptionsOf(result.Input)
	if opts.Mode == nil || *opts.Mode != types.LayoutModeSplit {
		t.Errorf("expected split mode from repo config, got %v", opts.Mode)
	}
}

func TestTomlConfigCustomThemes(t *testing.T) {
	tmp := t.TempDir()

	repoUnkDir := filepath.Join(tmp, ".unk")
	if err := os.MkdirAll(repoUnkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tomlContent := `
theme = "my-custom"

[themes.my-custom]
panel = "#001122"
accent = "#0088ff"
`
	if err := os.WriteFile(filepath.Join(repoUnkDir, "config.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	result, err := Resolve(&types.VCSInput{}, tmp)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	opts := types.OptionsOf(result.Input)
	if opts.Theme == nil || *opts.Theme != "my-custom" {
		t.Errorf("theme: want my-custom, got %v", opts.Theme)
	}
	if p, ok := result.CustomPalettes["my-custom"]; !ok {
		t.Error("CustomPalettes: missing my-custom")
	} else if p.Panel != "#001122" {
		t.Errorf("my-custom.panel: want #001122, got %q", p.Panel)
	}
}

func TestYAMLOverridesToml(t *testing.T) {
	// YAML at the same directory level should override TOML settings.
	tmp := t.TempDir()

	repoUnkDir := filepath.Join(tmp, ".unk")
	if err := os.MkdirAll(repoUnkDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// TOML sets mode=split
	tomlContent := `mode = "split"` + "\n"
	if err := os.WriteFile(filepath.Join(repoUnkDir, "config.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	// YAML overrides mode=stack
	yamlContent := "mode: stack\n"
	if err := os.WriteFile(filepath.Join(repoUnkDir, "config.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	result, err := Resolve(&types.VCSInput{}, tmp)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	opts := types.OptionsOf(result.Input)
	if opts.Mode == nil || *opts.Mode != types.LayoutModeStack {
		t.Errorf("mode: want stack (YAML overrides TOML), got %v", opts.Mode)
	}
}
