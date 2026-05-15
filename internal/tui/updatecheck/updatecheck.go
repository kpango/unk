package updatecheck

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	distTagsURL        = "https://registry.npmjs.org/-/package/unkdiff/dist-tags"
	updateCheckTimeout = 5 * time.Second
	disableNoticeEnv   = "HUNK_DISABLE_UPDATE_NOTICE"
)

var (
	stableVersion    = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	prereleaseVersion = regexp.MustCompile(`^\d+\.\d+\.\d+-[0-9A-Za-z.-]+$`)
)

// NoticeMsg is dispatched when a newer version of unk is available.
type NoticeMsg string

// CheckForUpdateCmd returns a tea.Cmd that fetches npm dist-tags in the background
// and dispatches a NoticeMsg if a newer version is available.
func CheckForUpdateCmd(installedVersion string) tea.Cmd {
	if os.Getenv(disableNoticeEnv) != "" {
		return nil
	}
	if installedVersion == "" || installedVersion == "dev" {
		return nil
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, distTagsURL, nil)
		if err != nil {
			return nil
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			return nil
		}
		defer resp.Body.Close()

		var tags map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
			return nil
		}

		notice := selectUpdateNotice(installedVersion, tags)
		if notice == "" {
			return nil
		}
		return NoticeMsg(notice)
	}
}

// selectUpdateNotice returns a notice string if a newer version is available, empty otherwise.
func selectUpdateNotice(installed string, tags map[string]string) string {
	isPrerelease := prereleaseVersion.MatchString(installed)
	isStable := stableVersion.MatchString(installed)

	if !isStable && !isPrerelease {
		return ""
	}

	channel := "latest"
	if isPrerelease {
		channel = "beta"
	}

	latest, ok := tags[channel]
	if !ok || latest == "" {
		return ""
	}

	if semverGT(latest, installed) {
		return "unk " + latest + " available — visit github.com/modem-dev/unk/releases"
	}

	// Check cross-channel: if on prerelease but stable is newer, suggest stable.
	if isPrerelease {
		if stable, ok := tags["latest"]; ok && semverGT(stable, strings.Split(installed, "-")[0]) {
			return "unk " + stable + " stable available — visit github.com/modem-dev/unk/releases"
		}
	}

	return ""
}

// semverGT returns true if version a is strictly greater than version b.
// Only handles numeric semver segments (no prerelease compare).
func semverGT(a, b string) bool {
	if a == b {
		return false
	}
	aCore := strings.SplitN(a, "-", 2)[0]
	bCore := strings.SplitN(b, "-", 2)[0]
	aSegs := strings.Split(aCore, ".")
	bSegs := strings.Split(bCore, ".")
	for i := range 3 {
		av, bv := 0, 0
		if i < len(aSegs) {
			av, _ = strconv.Atoi(aSegs[i])
		}
		if i < len(bSegs) {
			bv, _ = strconv.Atoi(bSegs[i])
		}
		if av > bv {
			return true
		}
		if av < bv {
			return false
		}
	}
	return false
}
