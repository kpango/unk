package highlight

import (
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/kpango/unk/internal/syncmap"
)

// textLineCache is the private implementation of Cache.
type textLineCache struct {
	mu sync.RWMutex
	m  map[string]string
}

func (c *textLineCache) load(text string) (string, bool) {
	c.mu.RLock()
	v, ok := c.m[text]
	c.mu.RUnlock()
	return v, ok
}

func (c *textLineCache) store(text, value string) {
	c.mu.Lock()
	if c.m == nil {
		c.m = make(map[string]string, 64)
	}
	c.m[text] = value
	c.mu.Unlock()
}

// bgCacheMap holds per-bgKey textLineCache entries under a single language.
// Using map[string]*textLineCache + RWMutex avoids interface boxing that
// sync.Map incurs when the key is a non-constant string, eliminating the
// per-call allocation that the old flat lang+"|"+bgKey sync.Map had.
type bgCacheMap struct {
	mu sync.RWMutex
	m  map[string]*textLineCache
}

func (b *bgCacheMap) get(bgKey string) *textLineCache {
	b.mu.RLock()
	c := b.m[bgKey]
	b.mu.RUnlock()
	return c
}

func (b *bgCacheMap) getOrCreate(bgKey string) *textLineCache {
	b.mu.RLock()
	c := b.m[bgKey]
	b.mu.RUnlock()
	if c != nil {
		return c
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if c = b.m[bgKey]; c != nil {
		return c
	}
	c = &textLineCache{}
	if b.m == nil {
		b.m = make(map[string]*textLineCache, 4)
	}
	b.m[bgKey] = c
	return c
}

// cachedLexer wraps a coalesced chroma.Lexer, or nil if the language is unknown.
// The pointer wrapper lets syncmap.Map distinguish "cached nil" from "not present".
type cachedLexer struct {
	lexer chroma.Lexer
}

var lexerCache = syncmap.New[string, *cachedLexer]()

func coalescedLexer(langID string) chroma.Lexer {
	if cl, ok := lexerCache.Load(langID); ok {
		return cl.lexer
	}
	var cl chroma.Lexer
	if l := lexers.Get(langID); l != nil {
		cl = chroma.Coalesce(l)
	}
	lexerCache.Store(langID, &cachedLexer{cl})
	return cl
}
