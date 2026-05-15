package model

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/kpango/unk/internal/diff"
	"github.com/kpango/unk/internal/loader"
	tuimsg "github.com/kpango/unk/internal/tui/msg"
	"github.com/kpango/unk/internal/tui/render"
	"github.com/kpango/unk/internal/tui/updatecheck"
)

// Init starts background tasks needed before the first render.
func (m *model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.watchEnabled {
		cmds = append(cmds, m.startWatchPoller())
	}
	if cmd := updatecheck.CheckForUpdateCmd(m.bootstrap.Version); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if m.isLoading {
		cmds = append(cmds, m.cmdLoadChangeset())
	} else {
		cmds = append(cmds, m.cmdBuildIntraCache())
	}
	if m.ipcCh != nil {
		cmds = append(cmds, m.cmdWaitIPC())
	}
	return tea.Batch(cmds...)
}

// cmdWaitIPC blocks until the next Cmd arrives on the IPC channel and delivers
// it as a tuimsg.IPC. Re-arm this cmd after every IPC in Update.
func (m *model) cmdWaitIPC() tea.Cmd {
	return func() tea.Msg {
		cmd, ok := <-m.ipcCh
		if !ok {
			return nil
		}
		return tuimsg.IPC{Cmd: cmd}
	}
}

// cmdLoadChangeset runs loader.LoadChangeset in the background and delivers
// tuimsg.ChangesetLoaded when done.
func (m *model) cmdLoadChangeset() tea.Cmd {
	bsCopy := m.bootstrap
	return func() tea.Msg {
		cwd := cwdOrEmpty()
		if err := loader.LoadChangeset(&bsCopy, cwd); err != nil {
			return tuimsg.ChangesetLoaded{Err: err}
		}
		return tuimsg.ChangesetLoaded{Bootstrap: &bsCopy}
	}
}

// cmdBuildIntraCache computes intra-line diff spans for all files off the UI
// goroutine and delivers tuimsg.IntraCacheReady.
func (m *model) cmdBuildIntraCache() tea.Cmd {
	files := m.bootstrap.Changeset.Files
	return func() tea.Msg {
		ic := make(map[string][2][][]diff.IntraSpan)
		render.BuildIntraCache(ic, files)
		return tuimsg.IntraCacheReady(ic)
	}
}
