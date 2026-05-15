package pager

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// LooksLikePatch reports whether text appears to be a unified diff.
func LooksLikePatch(text string) bool {
	normalized := StripANSI(text)
	return HasPrefixLine(normalized, "diff --git ") ||
		(HasPrefixLine(normalized, "--- ") && HasPrefixLine(normalized, "+++ ")) ||
		HasPrefixLine(normalized, "@@ ")
}

// StripANSI removes terminal control sequences from s.
func StripANSI(s string) string {
	var out strings.Builder
	out.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) {
			switch s[i+1] {
			case 'P', ']': // OSC / DCS — skip until ST or BEL
				end := strings.IndexAny(s[i:], "\x07\x1b\\")
				if end < 0 {
					i = len(s)
				} else {
					i += end + 1
				}
				continue
			case '[': // CSI
				j := i + 2
				for j < len(s) && (s[j] < '@' || s[j] > '~') {
					j++
				}
				if j < len(s) {
					j++
				}
				i = j
				continue
			default:
				i += 2
				continue
			}
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

// HasPrefixLine reports whether s starts with prefix or contains a line starting with prefix.
func HasPrefixLine(s, prefix string) bool {
	return strings.HasPrefix(s, prefix) || strings.Contains(s, "\n"+prefix)
}

// RunPlainText pipes text through $UNK_TEXT_PAGER / $HUNK_TEXT_PAGER / $PAGER / less -R.
func RunPlainText(text string) error {
	pagerCmd := os.Getenv("UNK_TEXT_PAGER")
	if pagerCmd == "" {
		pagerCmd = os.Getenv("HUNK_TEXT_PAGER") // legacy name
	}
	if pagerCmd == "" {
		pagerCmd = os.Getenv("PAGER")
	}
	if pagerCmd == "" || strings.Contains(pagerCmd, "unk") {
		pagerCmd = "less -R"
	}

	fi, err := os.Stdout.Stat()
	if err == nil && fi.Mode()&os.ModeCharDevice == 0 {
		_, err = fmt.Fprint(os.Stdout, text)
		return err
	}

	cmd := exec.Command("sh", "-c", pagerCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("pager %q: %w", pagerCmd, err)
	}
	_, _ = io.WriteString(stdin, text)
	stdin.Close()
	return cmd.Wait()
}
