package model

import (
	"github.com/kpango/unk/internal/tui/clipboard"
	"github.com/kpango/unk/internal/tui/patch"
)

// cmdYankUnk copies the text of the currently selected unk to the system
// clipboard using xclip, xsel, wl-copy, or pbcopy (whichever is available).
// Returns nil if no file/unk is selected or no clipboard tool is found.
func (m *model) cmdYankUnk() (string, error) {
	f := m.selectedFile()
	if f == nil || f.Patch == "" {
		return "", nil
	}
	text := patch.ExtractText(f.Patch, m.selectedUnkIndex)
	if text == "" {
		return "", nil
	}
	if err := clipboard.Write(text); err != nil {
		return "", err
	}
	return text, nil
}
