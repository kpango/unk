// Package msg defines the internal BubbleTea message types that flow through
// the unk TUI event loop. Placing them here decouples message dispatch sites
// (goroutines started in init.go / prerender.go) from the model package itself.
package msg

import (
	"time"

	"github.com/kpango/unk/internal/diff"
	"github.com/kpango/unk/internal/ipc"
	"github.com/kpango/unk/internal/types"
)

// ScrollTick drives the kinetic-scroll animation loop. One is always in-flight
// while momentum is nonzero; the handler re-arms it until velocity decays.
type ScrollTick struct{}

// KeyScrollTick is delivered every keyScrollInterval while a J/K key is held.
// Gen is compared against the model's keyScrollGen; stale ticks are dropped.
type KeyScrollTick struct {
	Gen uint64
	Dir int // +1 down, -1 up
}

// KeyScrollEnd is the watchdog timer that stops the key-scroll loop when the key
// is released. Only the latest watchdog (highest Epoch) can stop the loop.
type KeyScrollEnd struct{ Epoch uint64 }

// IPC wraps a command delivered from the Unix socket server.
type IPC struct{ Cmd ipc.Cmd }

// WatchTick is sent on each watch-poll interval.
type WatchTick struct{ At time.Time }

// WatchReload carries a hot-reloaded changeset and its pre-built intra-line diff
// cache, both produced off the UI goroutine in the watch tick handler.
type WatchReload struct {
	Changeset  types.Changeset
	IntraCache map[string][2][][]diff.IntraSpan
}

// ChangesetLoaded is delivered when the background cmdLoadChangeset goroutine
// finishes. Bootstrap is nil on error.
type ChangesetLoaded struct {
	Bootstrap *types.Bootstrap
	Err       error
}

// IntraCacheReady is delivered when the async BuildIntraCache goroutine completes.
// Assigning the cache and clearing the render cache triggers a second prewarm.
type IntraCacheReady map[string][2][][]diff.IntraSpan

// EditorFinished is delivered when the external editor process exits.
type EditorFinished struct{ Err error }
