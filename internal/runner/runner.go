package runner

import (
	"context"
	"net/http"
	_ "net/http/pprof" // register pprof handlers on DefaultServeMux
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kpango/unk/internal/ipc"
	"github.com/kpango/unk/internal/loader"
	"github.com/kpango/unk/internal/terminal"
	"github.com/kpango/unk/internal/tui"
	"github.com/kpango/unk/internal/types"
)

// Run starts the TUI for the given input. version is injected from the binary's
// build-time Version variable so this package stays import-cycle free.
func Run(ctx context.Context, input types.CLIInput, version string) error {
	if addr := os.Getenv("HUNK_PPROF_ADDR"); addr != "" {
		go func() { _ = http.ListenAndServe(addr, nil) }()
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Start the terminal background probe immediately; it usually responds in
	// < 5 ms and we collect the result before starting the TUI.
	type themeResult struct{ mode *types.TerminalThemeMode }
	themeCh := make(chan themeResult, 1)
	go func() {
		var mode *types.TerminalThemeMode
		if fi, err := os.Stdout.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
			mode = terminal.ProbeThemeMode()
		}
		themeCh <- themeResult{mode}
	}()

	bootstrap, err := loader.LoadConfig(input, loader.Options(cwd))
	if err != nil {
		return err
	}

	if tr := <-themeCh; tr.mode != nil {
		bootstrap.InitialThemeMode = tr.mode
	}

	if bootstrap.InitialTheme != nil && *bootstrap.InitialTheme == "auto" {
		resolved := "graphite"
		if bootstrap.InitialThemeMode != nil && *bootstrap.InitialThemeMode == types.ThemeModeLight {
			resolved = "paper"
		}
		bootstrap.InitialTheme = &resolved
	}

	bootstrap.Version = version

	ipcCh := make(chan ipc.Cmd, 16)
	go ipc.Serve(ctx, ipc.SocketPath(os.Getpid()), ipcCh)

	m := tui.New(*bootstrap, tui.WithIPCChannel(ipcCh))

	opts := []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithContext(ctx),
		tea.WithOutput(&syncWriter{w: os.Stdout}),
	}
	// When stdin is not a terminal (e.g. pager mode with piped git output),
	// explicitly open /dev/tty for keyboard input so BubbleTea never tries to
	// read from the exhausted pipe fd.
	if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
		opts = append(opts, tea.WithInputTTY())
	}

	p := tea.NewProgram(m, opts...)
	_, err = p.Run()
	return err
}

// syncWriter wraps *os.File and surrounds each write with DEC private mode 2026
// (synchronized output) markers so the terminal paints each frame atomically.
// It implements term.File (io.ReadWriteCloser + Fd) so BubbleTea v1.x can detect
// the terminal size via TIOCGWINSZ — without Fd(), ttyOutput stays nil and
// WindowSizeMsg is never sent, leaving termWidth=0 and View() returning "".
type syncWriter struct{ w *os.File }

func (s *syncWriter) Write(p []byte) (n int, err error) {
	const begin = "\033[?2026h"
	const end = "\033[?2026l"
	buf := make([]byte, 0, len(begin)+len(p)+len(end))
	buf = append(buf, begin...)
	buf = append(buf, p...)
	buf = append(buf, end...)
	if _, err = s.w.Write(buf); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *syncWriter) Read(p []byte) (int, error) { return s.w.Read(p) }
func (s *syncWriter) Close() error               { return nil }
func (s *syncWriter) Fd() uintptr                { return s.w.Fd() }
