package highlight

import (
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/lipgloss"

	"github.com/kpango/unk/internal/pool"
	"github.com/kpango/unk/internal/syncmap"
)

// builderPool pools *strings.Builder to eliminate per-line buffer allocations
// in the cold-render path of Line(). Builders larger than 64 KiB are discarded.
var builderPool = pool.New(func() *strings.Builder { return new(strings.Builder) })

func acquireBuilder() *strings.Builder {
	b := builderPool.Get()
	b.Reset()
	return b
}

func releaseBuilder(b *strings.Builder) {
	if b.Cap() <= 1<<16 {
		builderPool.Put(b)
	}
}

// style is the private implementation of Style. It caches per-token-type
// foreground hex colors and fully-rendered lines for a named chroma style.
//
// langCaches maps lang → *bgCacheMap → *textLineCache. Two-level map with
// plain map[string] + RWMutex at each level avoids the string-boxing allocation
// that sync.Map incurs for non-constant string keys, making CacheFor zero-alloc
// on the hot path (after the first access per lang/bg combination).
type style struct {
	cs          *chroma.Style
	tokenColors syncmap.Map[chroma.TokenType, string] // token type → hex color (or "")
	fgAnsi      syncmap.Map[chroma.TokenType, string] // token type → ANSI fg sequence (or "")
	bgAnsi      syncmap.Map[string, string]            // bg hex → ANSI bg sequence (or "")
	langMu      sync.RWMutex
	langCaches  map[string]*bgCacheMap // lang → per-bg cache map
}

// hexNibble converts a hex digit character to its 4-bit value.
func hexNibble(c byte) uint8 {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

// writeUint8Dec writes a uint8 decimal value into b starting at offset,
// returning the number of bytes written. Zero allocations.
func writeUint8Dec(b []byte, offset int, v uint8) int {
	if v >= 100 {
		b[offset] = '0' + v/100
		b[offset+1] = '0' + (v/10)%10
		b[offset+2] = '0' + v%10
		return 3
	}
	if v >= 10 {
		b[offset] = '0' + v/10
		b[offset+1] = '0' + v%10
		return 2
	}
	b[offset] = '0' + v
	return 1
}

// hexToRawANSI builds an ANSI 24-bit color sequence from a "#RRGGBB" hex string.
// prefix is either "38" (foreground) or "48" (background).
// Returns "" for unrecognized color formats.
func hexToRawANSI(hex, prefix string) string {
	if len(hex) != 7 || hex[0] != '#' {
		return ""
	}
	r := hexNibble(hex[1])<<4 | hexNibble(hex[2])
	g := hexNibble(hex[3])<<4 | hexNibble(hex[4])
	b := hexNibble(hex[5])<<4 | hexNibble(hex[6])
	// Max seq: "\x1b[48;2;255;255;255m" = 19 bytes
	var buf [20]byte
	buf[0] = '\x1b'
	buf[1] = '['
	n := 2
	n += copy(buf[n:], prefix)
	buf[n] = ';'; n++
	buf[n] = '2'; n++
	buf[n] = ';'; n++
	n += writeUint8Dec(buf[:], n, r)
	buf[n] = ';'; n++
	n += writeUint8Dec(buf[:], n, g)
	buf[n] = ';'; n++
	n += writeUint8Dec(buf[:], n, b)
	buf[n] = 'm'; n++
	return string(buf[:n])
}

func (s *style) colorFor(tt chroma.TokenType) string {
	if c, ok := s.tokenColors.Load(tt); ok {
		return c
	}
	entry := s.cs.Get(tt)
	c := ""
	if entry.Colour.IsSet() {
		c = entry.Colour.String()
	}
	s.tokenColors.Store(tt, c)
	return c
}

func (s *style) fgANSIFor(tt chroma.TokenType) string {
	if seq, ok := s.fgAnsi.Load(tt); ok {
		return seq
	}
	seq := hexToRawANSI(s.colorFor(tt), "38")
	s.fgAnsi.Store(tt, seq)
	return seq
}

func (s *style) bgANSIFor(bgHex string) string {
	if bgHex == "" {
		return ""
	}
	if seq, ok := s.bgAnsi.Load(bgHex); ok {
		return seq
	}
	seq := hexToRawANSI(bgHex, "48")
	s.bgAnsi.Store(bgHex, seq)
	return seq
}

func (s *style) innerCache(lang, bgKey string) *textLineCache {
	s.langMu.RLock()
	bg := s.langCaches[lang]
	s.langMu.RUnlock()
	if bg == nil {
		s.langMu.Lock()
		bg = s.langCaches[lang]
		if bg == nil {
			bg = &bgCacheMap{}
			if s.langCaches == nil {
				s.langCaches = make(map[string]*bgCacheMap, 4)
			}
			s.langCaches[lang] = bg
		}
		s.langMu.Unlock()
	}
	return bg.getOrCreate(bgKey)
}

// CacheFor pre-resolves the inner line cache for a given (lang, bg) pair.
// After the first call per (lang, bg) combination this is a zero-alloc hot path:
// two map[string] lookups under RLock, no key construction, no interface boxing.
func (s *style) CacheFor(langID string, bg lipgloss.Color) Cache {
	bgKey := string(bg)
	s.langMu.RLock()
	bm := s.langCaches[langID]
	s.langMu.RUnlock()
	if bm != nil {
		if c := bm.get(bgKey); c != nil {
			return c
		}
	}
	return s.innerCache(langID, bgKey)
}

// expandTabsLine expands tab characters to spaces using 4-column tab stops.
// Only allocates if the string contains tab characters. Used on cache-miss
// path inside Line so the hot path (cache hit) remains allocation-free.
func expandTabsLine(s string) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s) * 2)
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := 4 - (col % 4)
			for range spaces {
				sb.WriteByte(' ')
			}
			col += spaces
		} else {
			sb.WriteRune(r)
			col++
		}
	}
	return sb.String()
}
