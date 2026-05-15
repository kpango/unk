package model

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kpango/unk/internal/diff"
	"github.com/kpango/unk/internal/loader"
	tuimsg "github.com/kpango/unk/internal/tui/msg"
	"github.com/kpango/unk/internal/tui/render"
	"github.com/kpango/unk/internal/types"
)

// startWatchPoller polls for VCS changes at a 250 ms interval.
func (m *model) startWatchPoller() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return tuimsg.WatchTick{At: t}
	})
}

// handleWatchTick checks for changes and reloads when the working tree differs.
// Both loader.Load and render.BuildIntraCache run off the UI goroutine.
func (m *model) handleWatchTick() tea.Cmd {
	return func() tea.Msg {
		cwd := cwdOrEmpty()
		newBootstrap, err := loader.Load(m.bootstrap.Input, loader.Options(cwd))
		if err != nil {
			return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg { return tuimsg.WatchTick{At: t} })
		}
		if changesetDiffers(m.bootstrap.Changeset, newBootstrap.Changeset) {
			ic := make(map[string][2][][]diff.IntraSpan)
			render.BuildIntraCache(ic, newBootstrap.Changeset.Files)
			return tuimsg.WatchReload{Changeset: newBootstrap.Changeset, IntraCache: ic}
		}
		return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg { return tuimsg.WatchTick{At: t} })
	}
}

// changesetDiffers does a quick structural comparison to avoid redundant reloads.
func changesetDiffers(a, b types.Changeset) bool {
	if len(a.Files) != len(b.Files) {
		return true
	}
	for i := range a.Files {
		fa, fb := a.Files[i], b.Files[i]
		if fa.Path != fb.Path || fa.Stats != fb.Stats {
			return true
		}
		if fa.Metadata.CacheKey != fb.Metadata.CacheKey {
			return true
		}
	}
	return false
}

func cwdOrEmpty() string {
	cwd, _ := os.Getwd()
	return cwd
}
