package model

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	tuimsg "github.com/kpango/unk/internal/tui/msg"
	"github.com/kpango/unk/internal/tui/patch"
)

// cmdOpenInEditor suspends the TUI, opens $EDITOR at the selected file and
// unk start line, then resumes. Falls back to $VISUAL then "vi" if EDITOR
// is unset.
func (m *model) cmdOpenInEditor() tea.Cmd {
	f := m.selectedFile()
	if f == nil || f.Path == "" {
		return nil
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}
	lineNum := patch.NewStartLine(f.Patch, m.selectedUnkIndex)
	var args []string
	if lineNum > 0 {
		args = append(args, fmt.Sprintf("+%d", lineNum))
	}
	args = append(args, f.Path)
	c := exec.Command(editor, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return tuimsg.EditorFinished{Err: err}
	})
}
