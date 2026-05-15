package loader

import (
	"testing"

	"github.com/kpango/unk/internal/types"
)

func TestDetectLanguage(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		// existing extensions
		{"main.go", "go"},
		{"app.ts", "typescript"},
		{"app.tsx", "typescriptreact"},
		{"styles.scss", "scss"},
		// newly added extensions
		{"app.vue", "vue"},
		{"app.svelte", "svelte"},
		{"server.php", "php"},
		{"script.pl", "perl"},
		{"app.clj", "clojure"},
		{"Main.scala", "scala"},
		{"build.gradle", "groovy"},
		{"styles.less", "less"},
		{"styles.sass", "sass"},
		{"schema.graphql", "graphql"},
		{"schema.proto", "protobuf"},
		{"config.xml", "xml"},
		{"app.dart", "dart"},
		{"main.hs", "haskell"},
		{"main.ex", "elixir"},
		{"main.elm", "elm"},
		// basename-only files
		{"Dockerfile", "dockerfile"},
		{"Makefile", "makefile"},
		{"Gemfile", "ruby"},
		// variant: "Dockerfile.dev"
		{"Dockerfile.dev", "dockerfile"},
		// unknown
		{"binary.bin", ""},
	}
	for _, c := range cases {
		got := detectLanguage(c.path)
		if c.want == "" {
			if got != nil {
				t.Errorf("detectLanguage(%q): want nil, got %q", c.path, *got)
			}
			continue
		}
		if got == nil {
			t.Errorf("detectLanguage(%q): want %q, got nil", c.path, c.want)
			continue
		}
		if *got != c.want {
			t.Errorf("detectLanguage(%q): want %q, got %q", c.path, c.want, *got)
		}
	}
}

func TestCanReloadInput(t *testing.T) {
	text := "some patch"
	file := "my.patch"
	stdin := "-"

	cases := []struct {
		name  string
		input types.CLIInput
		want  bool
	}{
		{"vcs input", &types.VCSInput{}, true},
		{"show input", &types.ShowInput{}, true},
		{"patch with text (stdin already read)", &types.PatchInput{Text: &text}, false},
		{"patch with stdin file path", &types.PatchInput{File: &stdin}, false},
		{"patch with no file (stdin)", &types.PatchInput{}, false},
		{"patch with real file", &types.PatchInput{File: &file}, true},
	}
	for _, c := range cases {
		got := CanReloadInput(c.input)
		if got != c.want {
			t.Errorf("CanReloadInput(%s): want %v, got %v", c.name, c.want, got)
		}
	}
}
