package model

import (
	"bytes"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kpango/unk/internal/diff"
	"github.com/kpango/unk/internal/ipc"
	"github.com/kpango/unk/internal/loader"
	"github.com/kpango/unk/internal/tui/keys"
	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/tui/sidebar"
	"github.com/kpango/unk/internal/tui/styles"
	"github.com/kpango/unk/internal/types"
)

// FocusArea identifies which pane currently has keyboard focus.
type FocusArea string

const (
	FocusFiles   FocusArea = "files"
	FocusFilter  FocusArea = "filter"
	FocusSearch  FocusArea = "search"
	FocusCommand FocusArea = "command"
)

// Model is the root Bubbletea model for the unk TUI.
type model struct {
	// --- bootstrap ---
	bootstrap   types.Bootstrap
	keys        keys.KeyMap
	keymapStyle string // "helix", "vim", or "emacs"

	// --- layout & display toggles ---
	layoutMode       types.LayoutMode
	themeID          string
	showAgentNotes   bool
	showLineNumbers  bool
	wrapLines        bool
	showUnkHeaders   bool
	sidebarVisible   bool
	forceSidebarOpen bool

	// --- sidebar sizing ---
	sidebarWidth       int
	sidebarDragging    bool
	sidebarDragAnchorX int
	sidebarDragStartW  int

	// --- scrollbar drag ---
	scrollbarDragging      bool
	scrollbarDragAnchorY   int
	scrollbarDragScrollTop int

	// --- scroll momentum ---
	// scrollVelocity is the current kinetic velocity in lines/frame (signed).
	// scrollFrac accumulates the fractional part so sub-line motion is never lost.
	// scrollTicking is true while a scrollTickMsg is in flight; only one is ever
	// queued at a time so the animation loop is a simple re-arm chain.
	scrollVelocity float64
	scrollFrac     float64
	scrollTicking  bool

	// --- key-scroll (J/K hold) ---
	// keyScrollGen tracks the active tick loop; bumped only on first press or
	// direction change. Ticks whose gen doesn't match are dropped.
	// keyScrollWdEpoch is bumped on every key event (first press and each OS
	// repeat); only the watchdog with the LATEST epoch can stop the loop.
	// This prevents earlier watchdogs — launched before the most recent key
	// event arrived — from stopping the loop prematurely.
	// keyScrollPhase: 0=idle, 1=running. keyScrollDir: +1 or -1.
	keyScrollGen     uint64
	keyScrollWdEpoch uint64
	keyScrollPhase   int
	keyScrollDir     int

	// --- horizontal scroll ---
	codeHorizontalOffset int

	// --- navigation ---
	selectedFileIndex int
	selectedUnkIndex  int
	scrollTop         int

	// --- filter ---
	focusArea FocusArea
	filter    textinput.Model
	showHelp  bool
	helpPage  int // 0=Keys 1=Commands 2=Modes (cycled with ←/→)

	// --- keymap list overlay ---
	showKeymapList bool
	keymapListIdx  int

	// --- grep / content search ---
	search         textinput.Model
	grepQuery      string // committed search pattern (empty = no active search)
	grepMatchLines []int  // absolute diff-line numbers of matching lines (after prewarm)
	grepMatchIdx   int    // index into grepMatchLines for n/N navigation

	// --- command mode (:) ---
	cmdInput textinput.Model

	// --- terminal dimensions ---
	termWidth  int
	termHeight int

	// --- render caches (invalidated on changeset/layout/theme change) ---
	// intraCache stores pre-computed intra-line diff spans keyed by file CacheKey.
	// Each entry is [2][][]diff.IntraSpan: index 0=del spans, 1=add spans.
	// The inner slice is indexed by patch-line position (dense array avoids map overhead).
	intraCache map[string][2][][]diff.IntraSpan

	// patchLinesCache stores pre-split patch lines for each file, keyed by CacheKey
	// (or ID). Patch content is immutable for a given key, so this is safe to reuse
	// across renders. nil between clearRenderCache and first split (lazy init).
	// Background render goroutines (renderClone) do NOT share this; isMainGoroutine
	// (at end of struct) distinguishes main-goroutine model instances from clones.
	patchLinesCache map[string][]string

	// sectionCache stores pre-rendered file section strings keyed by sectionCacheKey.
	// It is cleared whenever any display state that affects rendering changes.
	// This eliminates repeated lipgloss + syntax-highlight work for unchanged files.
	sectionCache map[string]string

	// sectionLineCache caches the line count for each section in sectionCache.
	// Populated alongside sectionCache in the render.PrewarmMsg handler. Eliminates
	// O(section_size) strings.Count scans that would otherwise run on every scroll step
	// in renderDiffPane and totalDiffLines for every file above the viewport.
	sectionLineCache map[string]int

	// sectionLineOffsets caches a []int32 of byte-start positions per line for
	// each section in sectionCache. Populated alongside sectionLineCache so that
	// renderDiffPane can jump to any line in O(1) instead of scanning from the
	// beginning of the section string on every scroll step.
	sectionLineOffsets map[string][]int32

	// fileViewCache is a precomputed per-file array of section data for renderDiffPane.
	// Rebuilt lazily when fileViewDirty=true. Eliminates O(N) hash-map lookups per scroll
	// frame by resolving sectionCacheKey, rendered string, offsets, and lineCount once.
	// fileViewStarts[i] holds the cumulative line start of file i so binary search can
	// find the sticky-header file in O(log N) instead of O(N).
	fileViewCache  []fileView
	fileViewStarts []int // len = len(fileViewCache)+1; starts[i+1]-starts[i] = fileViewCache[i].lineCount
	fileViewDirty  bool

	// sidebarEntries caches the grouped entry list produced by sidebar.BuildEntries.
	// Rebuilt when the file list structure changes (filter, reload) or sidebar width changes.
	sidebarEntries      []sidebar.Entry
	sidebarEntriesDirty bool
	sidebarEntriesWidth int // sidebar width used when building entry row strings

	// sidebarRowsCache stores the pre-rendered sidebar file rows (excluding the
	// filter input line). Invalidated by clearRenderCache or markSidebarDirty.
	// The filter line is always rendered fresh to handle cursor-blink state.
	sidebarRowsCache string
	sidebarRowsDirty bool

	// menuBarCache stores the pre-rendered menu bar string.
	// Invalidated by clearRenderCache (theme/size/reload) and markSidebarDirty
	// (filter changes affect visible-file stats shown in the menu bar).
	menuBarCache string
	menuBarDirty bool

	// statusBarCache stores the pre-rendered status bar for non-FocusFilter frames.
	// Invalidated by clearRenderCache, markSidebarDirty (filter value change),
	// and updateNoticeMsg. When focusArea==FocusFilter the cache is always bypassed
	// because the cursor blink changes every frame; setting statusBarDirty=true in
	// that branch ensures a fresh render on the next FocusFiles frame.
	statusBarCache string
	statusBarDirty bool

	// divCharCache holds the ANSI-rendered "│" divider, computed lazily in renderBody
	// and invalidated (set to "") by clearRenderCache on theme or size change.
	divCharCache string

	// cachedPadLine holds the blank padding line ("<DiffPaneWidth spaces>\n"), computed
	// lazily in renderDiffPane and invalidated by clearRenderCache on size change.
	cachedPadLine string

	// diffPaneCache stores the last-rendered diff pane string together with the
	// scrollTop position it was built for. On cursor-blink renders (where scrollTop
	// and sectionCache haven't changed) View() reuses this string instead of
	// rebuilding the 34KB output. Invalidated by clearRenderCache (set to "").
	diffPaneScrollTop int
	diffPaneCache     string

	// bodyCache stores the last-rendered body (sidebar + divider + diffPane joined).
	// Valid when: focusArea==FocusFiles (filter cursor not animating), sidebar rows
	// not dirty, and diffPane not changed since last render. Avoids the joinColumns
	// allocation (~25KB) and renderSidebar re-render on every cursor-blink frame.
	// Cleared to "" by clearRenderCache; the conditions in renderBody guard its use.
	bodyCache string

	// prewarmGen is a monotonically increasing generation counter, incremented by
	// clearRenderCache. cmdPrewarmSections captures the current generation and the
	// render.PrewarmMsg handler discards results from stale generations, preventing stale
	// sections (rendered with a previous theme/layout) from entering sectionCache.
	prewarmGen uint64

	// pendingPrewarm is set to true by clearRenderCache and consumed by the outer
	// Update wrapper to auto-batch a cmdPrewarmSections on every cache invalidation.
	pendingPrewarm bool

	// prewarmPending counts how many prewarmMsgs are still expected for the current
	// generation. cmdPrewarmSections returns tea.Batch with one Cmd per file, so N
	// files → N prewarmMsgs. The outer Update wrapper renders progressively (one
	// frame per render.PrewarmMsg, rate-limited to 16ms intervals) so users see the first
	// real file section as soon as the fastest goroutine delivers it.
	// Reset to 0 by clearRenderCache; set to len(files) when prewarm is dispatched.
	prewarmPending int

	// lastPrewarmRender records when the most recent progressive prewarm render
	// was issued. Used by the rate-limiter in Update() to cap intermediate renders
	// to one per 16ms (≈60fps), preventing CPU thrashing when many prewarmMsgs
	// arrive in a burst (all goroutines finish simultaneously on high-CPU machines).
	// Reset to zero by clearRenderCache so the first msg of each cycle always renders.
	lastPrewarmRender time.Time

	// --- render pipeline ---
	//
	// builtFrame is the latest complete terminal frame. View() returns it in O(1).
	// Update() rebuilds it synchronously on every event where sectionCache is
	// non-nil. Preserved across clearRenderCache() so the last-known-good frame
	// stays visible while prewarm rebuilds sections (avoids placeholder flashes).
	// Explicitly cleared only when frame dimensions change (window resize, layout
	// toggle) so View() falls back to an inline renderFull() for that one cycle.
	builtFrame string

	// loadingFrame caches the placeholder frame that View() renders when
	// sectionCache==nil (prewarm in-flight) and builtFrame has been cleared.
	// Once built for a given (layout, theme, contentVersion), it is reused on
	// every subsequent clearRenderCache+builtFrame="" cycle with the same
	// parameters — turning repeated worst-case View() calls into O(1) returns.
	// Invalidated implicitly: if loadingFrameLayout != m.layout, or
	// loadingFrameTheme != m.themeID, or loadingFrameCV != m.contentVersion,
	// View() rebuilds and re-caches.
	loadingFrame       string
	loadingFrameLayout layout.Layout
	loadingFrameTheme  string
	loadingFrameCV     int

	// contentVersion is incremented whenever the changeset (file list) changes.
	// Used as part of the loadingFrame cache key so stale frames (showing old
	// file paths / stats in the sidebar) are never returned after a hot-reload.
	contentVersion int

	// --- watch mode ---
	watchEnabled bool

	// --- layout geometry (derived, cached) ---
	layout layout.Layout

	// --- pager mode (sidebar/menubar hidden) ---
	pagerMode bool

	// --- menu overlay (F10) ---
	menuOpen      bool
	activeMenuID  string // "file" or "view"
	menuItemIndex int

	// --- suppress-viewport-sync timer ---
	suppressViewportSyncUntil time.Time

	// --- startup update notice ---
	updateNotice string

	// --- IPC channel (Unix socket → BubbleTea) ---
	// Nil when IPC is disabled.
	ipcCh <-chan ipc.Cmd

	// --- loading state ---
	// isLoading is true from New() until changesetLoadedMsg arrives. During this
	// window the TUI shows a loading screen instead of diff content.
	isLoading bool
	loadErr   string // non-empty when the initial load failed

	// Model-local assembly buffers and flat byte slabs avoid sync.Pool round-trips
	// (lfstack contention is expensive at 128+ goroutines). Placed at the END of the
	// struct so they don't shift the memory layout of rendering-critical fields above
	// (layout, intraCache, patchLinesCache, etc.) — a mid-struct insertion would push
	// those fields into different cache lines, degrading hot-path rendering.
	// Not copied in renderClone — the clone gets fresh zero-value buffers.
	sidebarFlatBuf  []byte       // flat slab: all row bytes packed contiguously (sidebarEntry offsets into this)
	sidebarBuf      bytes.Buffer // renderSidebarInner
	sidebarEntryBuf bytes.Buffer // sidebar entry row building (avoids Pool in entries-rebuild loop)
	menuBarBuf      bytes.Buffer // renderMenuBarInner
	statusBarBuf    bytes.Buffer // renderStatusBarInner
	scrollbarBuf    bytes.Buffer // renderScrollbar (avoids Pool)
	diffPaneBuf     bytes.Buffer // renderDiffPane
	bodyBuf         bytes.Buffer // renderBody (joinColumnsInto/joinColumnsAllInto)
	frameBuf        bytes.Buffer // renderFull
	placeholderBuf  bytes.Buffer // renderFileSectionPlaceholder

	// isMainGoroutine is true only in the real Model created by New(). It is NOT
	// set in renderClone, so background render goroutines use
	// the pooled patch-splitting path (which is also the safe concurrent path).
	// Placed here (after buffer block) so it doesn't shift layout layout.Layout above.
	isMainGoroutine bool

	// splitSavingsCache caches splitViewLineSavings(patch) results keyed by file
	// CacheKey. Patch content is immutable per key, so this survives clearRenderCache.
	// Only cleared on changeset reload (watchReloadMsg) when file keys change.
	// Placed here (after buffer block) so it doesn't shift layout layout.Layout above.
	splitSavingsCache map[string]int
}

// Option is a functional option for configuring a model.
type Option func(*model)

// WithIPCChannel sets the IPC channel for Unix-socket integration.
// When nil or not provided, IPC is disabled.
func WithIPCChannel(ch <-chan ipc.Cmd) Option {
	return func(m *model) { m.ipcCh = ch }
}

// New creates the root BubbleTea model from a Bootstrap.
func New(bootstrap types.Bootstrap, opts ...Option) tea.Model {
	cliOpts := types.OptionsOf(bootstrap.Input)

	mode := types.LayoutModeAuto
	if bootstrap.InitialMode != "" {
		mode = bootstrap.InitialMode
	}

	theme := "graphite"
	if bootstrap.InitialTheme != nil {
		theme = *bootstrap.InitialTheme
	}

	lineNumbers := true
	if bootstrap.InitialShowLineNumbers != nil {
		lineNumbers = *bootstrap.InitialShowLineNumbers
	}

	wrap := false
	if bootstrap.InitialWrapLines != nil {
		wrap = *bootstrap.InitialWrapLines
	}

	unkHeaders := true
	if bootstrap.InitialShowUnkHeaders != nil {
		unkHeaders = *bootstrap.InitialShowUnkHeaders
	}

	agentNotes := false
	if bootstrap.InitialShowAgentNotes != nil {
		agentNotes = *bootstrap.InitialShowAgentNotes
	}

	watchEnabled := cliOpts.Watch != nil && *cliOpts.Watch && loader.CanReloadInput(bootstrap.Input)

	// In pager mode the sidebar and menu bar start hidden.
	isPagerMode := cliOpts.Pager != nil && *cliOpts.Pager
	sidebarVisible := !isPagerMode

	fi := textinput.New()
	fi.Placeholder = "filter files…"

	si := textinput.New()
	si.Placeholder = "grep content…"

	ci := textinput.New()
	ci.Placeholder = ""

	// isLoading is true when the bootstrap has no changeset yet (the caller
	// used LoadConfig and will deliver the changeset via cmdLoadChangeset).
	// When the bootstrap already has files (direct construction in tests or
	// reload paths), isLoading stays false so the diff renders immediately.
	needsLoad := len(bootstrap.Changeset.Files) == 0 && bootstrap.Input != nil

	// Register any user-defined custom palettes before the first render so that
	// m.palette() and the theme cycle list include them.
	if len(bootstrap.CustomPalettes) > 0 {
		styles.RegisterCustomPalettes(bootstrap.CustomPalettes)
	}

	keymapStyle := "helix"
	if bootstrap.InitialKeymap != nil && *bootstrap.InitialKeymap != "" {
		keymapStyle = *bootstrap.InitialKeymap
	}
	switch keymapStyle {
	case "vim", "emacs":
		// keep
	default:
		keymapStyle = "helix"
	}

	// Build base keymap, then apply per-action overrides from config.
	km := keys.ApplyOverrides(keys.KeyMapForStyle(keymapStyle), bootstrap.KeyBindingOverrides)

	m := &model{
		bootstrap:           bootstrap,
		keys:                km,
		keymapStyle:         keymapStyle,
		layoutMode:          mode,
		themeID:             theme,
		showAgentNotes:      agentNotes,
		showLineNumbers:     lineNumbers,
		wrapLines:           wrap,
		showUnkHeaders:      unkHeaders,
		sidebarVisible:      sidebarVisible,
		sidebarWidth:        34,
		focusArea:           FocusFiles,
		filter:              fi,
		search:              si,
		cmdInput:            ci,
		intraCache:          make(map[string][2][][]diff.IntraSpan),
		patchLinesCache:     make(map[string][]string),
		splitSavingsCache:   make(map[string]int),
		isMainGoroutine:     true,
		sidebarEntriesDirty: true,
		sidebarRowsDirty:    true,
		menuBarDirty:        true,
		watchEnabled:        watchEnabled,
		pagerMode:           isPagerMode,
		isLoading:           needsLoad,
	}
	for _, o := range opts {
		o(m)
	}
	return m
}
