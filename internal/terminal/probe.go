//go:build !windows

package terminal

import (
	"os"
	"regexp"
	"strconv"
	"time"

	cterm "github.com/charmbracelet/x/term"
	"github.com/kpango/unk/internal/types"
	"golang.org/x/sys/unix"
)

const osc11Query = "\x1b]11;?\x1b\\"

// ProbeThemeMode sends an OSC 11 query to /dev/tty and returns the terminal
// background luminance class, or nil on any failure (150 ms timeout).
// Uses unix.Select for polling so no goroutine is leaked on timeout.
func ProbeThemeMode() *types.TerminalThemeMode {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil
	}
	defer f.Close()

	fd := f.Fd()
	if !cterm.IsTerminal(fd) {
		return nil
	}

	state, err := cterm.MakeRaw(fd)
	if err != nil {
		return nil
	}
	defer cterm.Restore(fd, state) //nolint:errcheck

	if _, err := f.WriteString(osc11Query); err != nil {
		return nil
	}

	deadline := time.Now().Add(150 * time.Millisecond)
	var acc []byte
	buf := make([]byte, 128)

	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		tv := unix.NsecToTimeval(remaining.Nanoseconds())
		var rfds unix.FdSet
		rfds.Set(int(fd))

		n, err := unix.Select(int(fd)+1, &rfds, nil, nil, &tv)
		if err == unix.EINTR {
			continue
		}
		if err != nil || n == 0 {
			return nil
		}

		n, err = f.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			if m := parseOSC11ThemeMode(string(acc)); m != nil {
				return m
			}
		}
		if err != nil {
			return nil
		}
	}

	return nil
}

var osc11RGBRe = regexp.MustCompile(`\x1b\]11;rgb:([0-9a-fA-F]{2,4})/([0-9a-fA-F]{2,4})/([0-9a-fA-F]{2,4})(?:\x07|\x1b\\)`)
var osc11HexRe = regexp.MustCompile(`\x1b\]11;#([0-9a-fA-F]{6})(?:\x07|\x1b\\)`)

func parseOSC11ThemeMode(s string) *types.TerminalThemeMode {
	var r, g, b uint8
	if m := osc11RGBRe.FindStringSubmatch(s); m != nil {
		r = parseHexChannel(m[1])
		g = parseHexChannel(m[2])
		b = parseHexChannel(m[3])
	} else if m := osc11HexRe.FindStringSubmatch(s); m != nil {
		hex := m[1]
		rv, _ := strconv.ParseUint(hex[0:2], 16, 8)
		gv, _ := strconv.ParseUint(hex[2:4], 16, 8)
		bv, _ := strconv.ParseUint(hex[4:6], 16, 8)
		r, g, b = uint8(rv), uint8(gv), uint8(bv)
	} else {
		return nil
	}
	mode := themeModeForBackground(r, g, b)
	return &mode
}

func parseHexChannel(s string) uint8 {
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0
	}
	max := uint64(1<<(4*len(s))) - 1
	return uint8((v * 255) / max)
}

func themeModeForBackground(r, g, b uint8) types.TerminalThemeMode {
	linearize := func(c uint8) float64 {
		n := float64(c) / 255.0
		if n <= 0.03928 {
			return n / 12.92
		}
		return ((n + 0.055) / 1.055) * ((n + 0.055) / 1.055)
	}
	lum := 0.2126*linearize(r) + 0.7152*linearize(g) + 0.0722*linearize(b)
	if lum > 0.5 {
		return types.ThemeModeLight
	}
	return types.ThemeModeDark
}
