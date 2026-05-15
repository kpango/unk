package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kpango/unk/internal/ipc"
	tuimodel "github.com/kpango/unk/internal/tui/model"
	"github.com/kpango/unk/internal/types"
)

// Option is a functional option for configuring the TUI model.
type Option = tuimodel.Option

// WithIPCChannel sets the IPC channel for Unix-socket integration.
func WithIPCChannel(ch <-chan ipc.Cmd) Option {
	return tuimodel.WithIPCChannel(ch)
}

// New creates the root BubbleTea model for the unk TUI.
func New(bootstrap types.Bootstrap, opts ...Option) tea.Model {
	return tuimodel.New(bootstrap, opts...)
}
