// Package highlight provides syntax highlighting for diff line content using
// chroma lexers. Each token segment sets both the diff-line background and the
// syntax foreground, so background colors persist across token boundaries.
package highlight

import (
	"github.com/alecthomas/chroma/v2"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"

	"github.com/kpango/unk/internal/syncmap"
)

// Cache is an opaque rendered-line cache for a fixed (lang, bg) pair.
// Obtained via Style.CacheFor; passed to LineFromCache.
// The interface is sealed — its methods are unexported so external packages
// cannot implement it, only hold and pass values of this type.
type Cache interface {
	load(string) (string, bool)
	store(string, string)
}

// Style is the interface for a syntax-highlighting style.
// Obtained via ForTheme; used to resolve line caches and render tokens.
type Style interface {
	// CacheFor pre-resolves the inner line cache for a given (lang, bg) pair.
	CacheFor(langID string, bg lipgloss.Color) Cache
	colorFor(tt chroma.TokenType) string
	fgANSIFor(tt chroma.TokenType) string
	bgANSIFor(bgHex string) string
	innerCache(lang, bgKey string) *textLineCache
}

// Option is a functional option for configuring a style.
type Option func(*style)

// unkThemeToChroma maps unk theme IDs to chroma style names.
var unkThemeToChroma = map[string]string{
	"graphite":         "dracula",
	"catppuccin-mocha": "catppuccin-mocha",
	"catppuccin-latte": "catppuccin-latte",
	"solarized-dark":   "solarized-dark",
	"solarized-light":  "solarized-light",
}

var styleCache = syncmap.New[string, Style]()

// ForTheme returns a cached highlight Style for the given unk theme ID.
func ForTheme(unkTheme string, opts ...Option) Style {
	if s, ok := styleCache.Load(unkTheme); ok {
		return s
	}
	name, ok := unkThemeToChroma[unkTheme]
	if !ok {
		name = "dracula"
	}
	cs := chromastyles.Get(name)
	if cs == nil {
		cs = chromastyles.Fallback
	}
	s := &style{
		cs:          cs,
		tokenColors: syncmap.New[chroma.TokenType, string](),
		fgAnsi:      syncmap.New[chroma.TokenType, string](),
		bgAnsi:      syncmap.New[string, string](),
	}
	for _, o := range opts {
		o(s)
	}
	styleCache.Store(unkTheme, s)
	return s
}

// Line syntax-highlights plain text in the given language and returns a string
// with embedded ANSI fg+bg codes. Results are memoized per (lang, bg, text)
// so repeated renders of the same line are O(1) map lookups with zero heap
// allocation on the hot path.
//
// text may contain raw tab characters; tabs are expanded to 4-column stops
// before tokenizing and the result is cached under the original (raw) key.
// Callers do NOT need to expand tabs before calling — the cache lookup uses
// the raw key, so cache hits are zero-alloc even for tab-containing lines.
//
// The two-level cache eliminates the per-call key-string construction overhead:
//   - Outer cache: (lang + "|" + bg) → Cache — looked up on first
//     access only; the caller can also pre-resolve via CacheFor.
//   - Inner cache: text → rendered string — map[string]string lookup with the
//     existing `text` string, zero allocations.
//
// Returns "" if the language is not recognized or tokenization fails — the
// caller should fall back to plain text rendering.
func Line(text, langID string, bg lipgloss.Color, hlStyle Style) string {
	if hlStyle == nil || langID == "" || text == "" {
		return ""
	}

	bgKey := string(bg) // lipgloss.Color is type string — zero-cost cast
	inner := hlStyle.innerCache(langID, bgKey)

	// Zero-alloc cache lookup: text is already a string, no key construction.
	// The cache is keyed by the raw (possibly tab-containing) string so that
	// callers can look up without expanding tabs on the hot path.
	if v, ok := inner.load(text); ok {
		return v
	}

	// Cache miss: expand tabs before tokenizing so spaces reach the terminal.
	// The expanded string is used only for tokenization; the result is stored
	// under the original (raw) key so future lookups with the same raw text hit.
	expanded := expandTabsLine(text)

	lexer := coalescedLexer(langID)
	if lexer == nil {
		return ""
	}
	iter, err := lexer.Tokenise(nil, expanded)
	if err != nil {
		return ""
	}
	tokens := iter.Tokens()
	if len(tokens) == 0 {
		return ""
	}

	bgSeq := hlStyle.bgANSIFor(bgKey)
	sb := acquireBuilder()
	defer releaseBuilder(sb)
	sb.Grow(len(expanded) + len(tokens)*8)

	// Emit ANSI sequences with minimal overhead:
	// - Background (bgSeq) is written once at the start of the first token.
	//   Subsequent fg changes use fg-only sequences without re-emitting bg.
	// - ESC[39m resets fg only (bg stays), avoiding a full reset + bg re-emit at
	//   every span boundary.
	// - When bgSeq is set, the line ends with ESC[39m (fg-only reset) rather than
	//   ESC[m so the background stays active for the caller to write padding spaces
	//   without needing to re-emit the background sequence. The caller is responsible
	//   for writing a final ESC[m after any trailing spaces.
	// - When bgSeq is empty, the line ends with ESC[m (full reset) as usual.
	// This produces significantly shorter cached strings than per-token resets,
	// and allows the caller to skip rawBase.writeOpen for padding spaces.
	lineOpen := false // whether bgSeq has been written for this line
	prevFg := "\x00" // sentinel: fg of the previous token
	for _, tok := range tokens {
		if tok.Type == chroma.EOFType || tok.Value == "" {
			continue
		}
		fg := hlStyle.fgANSIFor(tok.Type)
		if fg != prevFg {
			if !lineOpen {
				// First token: open bg (once for the whole line).
				if bgSeq != "" {
					sb.WriteString(bgSeq)
				}
				lineOpen = bgSeq != "" || fg != ""
			}
			if fg != "" {
				sb.WriteString(fg) // switch to new fg (bg stays)
			} else if prevFg != "\x00" && prevFg != "" {
				sb.WriteString("\x1b[39m") // reset fg only; bg remains active
			}
			prevFg = fg
		}
		sb.WriteString(tok.Value)
	}
	if lineOpen {
		if bgSeq != "" {
			// bg is active: fg-only reset keeps bg for the caller's padding spaces.
			sb.WriteString("\x1b[39m")
		} else {
			sb.WriteString("\x1b[m")
		}
	}

	result := sb.String()
	if result == "" {
		return ""
	}
	inner.store(text, result) // store under original (raw) key
	return result
}

// LineFromCache is the zero-alloc fast path when the caller has pre-resolved
// the inner cache via CacheFor. Returns "" on cache miss (caller must fall
// back to Line or plain rendering).
func LineFromCache(text string, cache Cache) (string, bool) {
	if cache == nil {
		return "", false
	}
	return cache.load(text)
}
