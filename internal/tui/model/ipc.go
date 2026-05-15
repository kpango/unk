package model

// ipc.go — handles inbound IPC commands from external tools (navigate, reload).

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kpango/unk/internal/ipc"
	"github.com/kpango/unk/internal/loader"
)

// handleIPCCmd applies a decoded IPC command to the model and returns any
// follow-up tea.Cmd needed (e.g. a reload trigger).
func (m *model) handleIPCCmd(cmd ipc.Cmd) tea.Cmd {
	switch cmd.Type {
	case ipc.CmdNavigate:
		if cmd.File == "" {
			return nil
		}
		files := m.visibleFiles()
		for i, f := range files {
			if f.Path != cmd.File {
				continue
			}
			m.selectedFileIndex = i
			hi := cmd.Unk - 1
			if hi < 0 {
				hi = 0
			}
			if n := len(f.Metadata.Unks); n > 0 && hi >= n {
				hi = n - 1
			}
			m.selectedUnkIndex = hi
			m.scrollTop = m.computeUnkScrollOffset()
			m.markSidebarDirty()
			break
		}

	case ipc.CmdReload:
		if loader.CanReloadInput(m.bootstrap.Input) {
			return m.handleWatchTick()
		}
	}
	return nil
}
