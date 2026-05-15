package clipboard

import (
	"os/exec"
	"strings"
)

// Write writes text to the system clipboard using the first available tool:
// wl-copy (Wayland), xclip, xsel (X11), or pbcopy (macOS).
// Returns nil when no clipboard tool is found rather than an error.
func Write(text string) error {
	for _, args := range [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
		{"pbcopy"},
	} {
		if path, err := exec.LookPath(args[0]); err == nil {
			cmd := exec.Command(path, args[1:]...)
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
	}
	return nil
}
