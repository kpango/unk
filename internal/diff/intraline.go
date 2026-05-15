package diff

import (
	"unsafe"

	"github.com/kpango/unk/internal/pool"
)

// IntraSpan is one contiguous run of characters with a "changed" flag.
type IntraSpan struct {
	Text    string
	Changed bool
}

// maxTokens caps token count per line in the LCS computation.
// maxDP is the maximum dp table size: (maxTokens+1)^2 entries.
// fastTokens/fastDP is the common-case size: lines with ≤ 20 tokens per side
// fit in a 441-int table (~3.5 KB) vs the 40401-int worst-case table (~323 KB).
// The pool allocates fastDP initially; IntraLineDiff grows dp lazily when needed.
const (
	fastTokens = 20
	fastDP     = (fastTokens + 1) * (fastTokens + 1) // 441
	maxTokens  = 200
	maxDP      = (maxTokens + 1) * (maxTokens + 1) // 40401
)

// workspace holds all scratch buffers needed by IntraLineDiff.
// A single pool entry replaces ~4 separate per-call allocations.
// dp starts small (fastDP ints ≈ 3.5 KB) and grows lazily to maxDP.
// Small entries survive GC collection much better than the old 323 KB entries.
type workspace struct {
	dp         []int    // LCS dynamic-programming table; starts at fastDP, grows on demand
	oldMatched []bool   // traceback results for old tokens; cap = maxTokens
	newMatched []bool   // traceback results for new tokens; cap = maxTokens
	oldTokens  []string // tokenized old line; grows on demand
	newTokens  []string // tokenized new line; grows on demand
}

var wsPool = pool.New(func() *workspace {
	return &workspace{
		dp:         make([]int, fastDP), // start small; grown lazily in IntraLineDiff
		oldMatched: make([]bool, maxTokens),
		newMatched: make([]bool, maxTokens),
		oldTokens:  make([]string, 0, 32),
		newTokens:  make([]string, 0, 32),
	}
})

// Workspace is an opaque scratch buffer for IntraLineDiffWith.
// Acquire one per goroutine/batch via AcquireWS and release via ReleaseWS.
// This avoids the per-call pool round-trip when computing many diffs in sequence.
type Workspace = workspace

// AcquireWS returns a pooled Workspace ready for use with IntraLineDiffWith.
func AcquireWS() *Workspace { return wsPool.Get() }

// ReleaseWS returns a Workspace to the pool. The caller must not use ws after
// calling ReleaseWS.
func ReleaseWS(ws *Workspace) { wsPool.Put(ws) }

// IntraLineDiff computes word-level differences between two lines and returns
// the annotated spans for each line. Changed words are marked with Changed=true.
// Returns nil, nil for lines that are too similar or too different to be useful.
//
// All scratch allocations (dp table, token slices, matched-bool arrays) are
// pooled so that the only per-call heap cost is the returned []IntraSpan slices.
//
// For batches of many line pairs, prefer IntraLineDiffWith to amortise the pool
// round-trip (one Get/Put per batch instead of one per pair).
func IntraLineDiff(oldLine, newLine string) ([]IntraSpan, []IntraSpan) {
	if oldLine == newLine {
		return nil, nil
	}
	ws := wsPool.Get()
	result1, result2 := intraLineDiff(ws, oldLine, newLine)
	wsPool.Put(ws)
	return result1, result2
}

// IntraLineDiffWith is the same as IntraLineDiff but uses the caller-supplied
// workspace ws instead of the pool. ws must have been obtained via AcquireWS.
// Use this inside loops to avoid a pool Get/Put on every iteration.
func IntraLineDiffWith(ws *Workspace, oldLine, newLine string) ([]IntraSpan, []IntraSpan) {
	if oldLine == newLine {
		return nil, nil
	}
	return intraLineDiff(ws, oldLine, newLine)
}

// IntraLineDiffInto appends spans for oldLine and newLine to the provided dst
// slices and returns the extended slices. It is the zero-alloc variant of
// IntraLineDiffWith: the caller owns one flat []IntraSpan backing buffer per
// side and grows it across many line pairs. Sub-slicing the returned slices
// at the boundary captured before the call gives per-pair span views that
// share the same backing array — one backing allocation per goroutine instead
// of one per line pair.
//
// When oldLine == newLine both dst slices are returned unchanged (no spans added).
func IntraLineDiffInto(ws *Workspace, oldDst, newDst []IntraSpan, oldLine, newLine string) ([]IntraSpan, []IntraSpan) {
	if oldLine == newLine {
		return oldDst, newDst
	}
	return intraLineDiffInto(ws, oldDst, newDst, oldLine, newLine)
}

// intraLineDiffInto is the shared implementation for IntraLineDiffInto.
// It appends spans to oldDst/newDst and returns the extended slices.
func intraLineDiffInto(ws *workspace, oldDst, newDst []IntraSpan, oldLine, newLine string) ([]IntraSpan, []IntraSpan) {
	ws.oldTokens = tokenizeInto(oldLine, ws.oldTokens[:0])
	ws.newTokens = tokenizeInto(newLine, ws.newTokens[:0])
	oldWords := ws.oldTokens
	newWords := ws.newTokens
	if len(oldWords) == 0 || len(newWords) == 0 {
		return oldDst, newDst
	}
	if len(oldWords) > maxTokens {
		oldWords = oldWords[:maxTokens]
	}
	if len(newWords) > maxTokens {
		newWords = newWords[:maxTokens]
	}
	n, m := len(oldWords), len(newWords)
	stride := m + 1
	needed := (n + 1) * stride
	if needed > cap(ws.dp) {
		ws.dp = make([]int, needed)
	}
	dp := ws.dp[:needed]
	clear(dp)
	oldMatched := ws.oldMatched[:n]
	newMatched := ws.newMatched[:m]
	clear(oldMatched)
	clear(newMatched)
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if oldWords[i-1] == newWords[j-1] {
				dp[i*stride+j] = dp[(i-1)*stride+(j-1)] + 1
			} else {
				dp[i*stride+j] = max(dp[(i-1)*stride+j], dp[i*stride+(j-1)])
			}
		}
	}
	i, j := n, m
	for i > 0 && j > 0 {
		if oldWords[i-1] == newWords[j-1] {
			oldMatched[i-1] = true
			newMatched[j-1] = true
			i--
			j--
		} else if dp[(i-1)*stride+j] > dp[i*stride+(j-1)] {
			i--
		} else {
			j--
		}
	}
	oldDst = buildSpansInto(oldDst, oldWords, oldMatched)
	newDst = buildSpansInto(newDst, newWords, newMatched)
	return oldDst, newDst
}

// intraLineDiff is the shared implementation for IntraLineDiff and IntraLineDiffWith.
func intraLineDiff(ws *workspace, oldLine, newLine string) ([]IntraSpan, []IntraSpan) {
	// Tokenize both lines into the workspace buffers.
	ws.oldTokens = tokenizeInto(oldLine, ws.oldTokens[:0])
	ws.newTokens = tokenizeInto(newLine, ws.newTokens[:0])

	oldWords := ws.oldTokens
	newWords := ws.newTokens

	if len(oldWords) == 0 || len(newWords) == 0 {
		return nil, nil
	}

	// Cap token slices at maxTokens.
	if len(oldWords) > maxTokens {
		oldWords = oldWords[:maxTokens]
	}
	if len(newWords) > maxTokens {
		newWords = newWords[:maxTokens]
	}
	n, m := len(oldWords), len(newWords)
	stride := m + 1
	needed := (n + 1) * stride

	// Grow dp lazily: start at fastDP (common case), expand to maxDP only when needed.
	if needed > cap(ws.dp) {
		ws.dp = make([]int, needed)
	}
	// Use pre-allocated dp table; clear only the used portion.
	dp := ws.dp[:needed]
	clear(dp)

	// Use pre-allocated matched arrays; clear only the used portion.
	oldMatched := ws.oldMatched[:n]
	newMatched := ws.newMatched[:m]
	clear(oldMatched)
	clear(newMatched)

	// Fill LCS dp table.
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if oldWords[i-1] == newWords[j-1] {
				dp[i*stride+j] = dp[(i-1)*stride+(j-1)] + 1
			} else {
				dp[i*stride+j] = max(dp[(i-1)*stride+j], dp[i*stride+(j-1)])
			}
		}
	}

	// Traceback: mark matched token indices.
	i, j := n, m
	for i > 0 && j > 0 {
		if oldWords[i-1] == newWords[j-1] {
			oldMatched[i-1] = true
			newMatched[j-1] = true
			i--
			j--
		} else if dp[(i-1)*stride+j] > dp[i*stride+(j-1)] {
			i--
		} else {
			j--
		}
	}

	// Build result spans. These allocate new slices that escape to the caller.
	oldSpans := buildSpans(oldWords, oldMatched)
	newSpans := buildSpans(newWords, newMatched)
	return oldSpans, newSpans
}

// tokenizeInto splits s into word/non-word token runs, appending to buf.
// Tokens are substrings of s — no extra copies.
func tokenizeInto(s string, buf []string) []string {
	if len(s) == 0 {
		return buf
	}
	isWord := func(r rune) bool {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
	}
	start := 0
	inWord := isWord(rune(s[0]))
	for i := 1; i < len(s); i++ {
		w := isWord(rune(s[i]))
		if w != inWord {
			buf = append(buf, s[start:i])
			start = i
			inWord = w
		}
	}
	return append(buf, s[start:])
}

// buildSpans converts a token slice and its match flags into IntraSpan runs.
// Adjacent tokens with the same Changed state are merged.
// concatTokens is used for multi-token spans instead of strings.Join to avoid
// the internal strings.Builder allocation that Join incurs.
func buildSpans(tokens []string, matched []bool) []IntraSpan {
	if len(tokens) == 0 {
		return nil
	}
	// Pre-count spans to allocate exact capacity.
	nSpans := 1
	for i := 1; i < len(tokens); i++ {
		if matched[i] != matched[i-1] {
			nSpans++
		}
	}
	spans := make([]IntraSpan, 0, nSpans)
	spanStart := 0
	changed := !matched[0]
	for i := 1; i < len(tokens); i++ {
		c := !matched[i]
		if c != changed {
			spans = append(spans, IntraSpan{
				Text:    concatTokens(tokens[spanStart:i]),
				Changed: changed,
			})
			spanStart = i
			changed = c
		}
	}
	spans = append(spans, IntraSpan{
		Text:    concatTokens(tokens[spanStart:]),
		Changed: changed,
	})
	return spans
}

// buildSpansInto is the flat-backing variant of buildSpans. It appends spans to
// dst instead of allocating a fresh slice, allowing the caller to use a single
// pre-allocated backing buffer across many line pairs. Returns the extended slice.
func buildSpansInto(dst []IntraSpan, tokens []string, matched []bool) []IntraSpan {
	if len(tokens) == 0 {
		return dst
	}
	spanStart := 0
	changed := !matched[0]
	for i := 1; i < len(tokens); i++ {
		c := !matched[i]
		if c != changed {
			dst = append(dst, IntraSpan{Text: concatTokens(tokens[spanStart:i]), Changed: changed})
			spanStart = i
			changed = c
		}
	}
	return append(dst, IntraSpan{Text: concatTokens(tokens[spanStart:]), Changed: changed})
}

// concatTokens joins adjacent token substrings into a single string without
// allocating. tokenizeInto produces tokens as consecutive substrings of the
// same original string, so the concatenation of any contiguous range
// tokens[i:j] is the re-slice tokens[i][:totalLen] — a zero-copy operation.
//
// Safety: all tokens are substrings of the same backing string s. Consecutive
// tokens satisfy end(tokens[k]) == start(tokens[k+1]). The result shares the
// same backing array as tokens[0] and is kept alive by the GC via tokens[0]'s
// pointer. The total length never exceeds len(s) - offset(tokens[0]).
func concatTokens(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	if len(tokens) == 1 {
		return tokens[0]
	}
	total := 0
	for _, t := range tokens {
		total += len(t)
	}
	// Extend tokens[0] to cover total bytes. All tokens are adjacent substrings
	// of the same string, so this is equivalent to re-slicing the original.
	type strHdr struct {
		ptr unsafe.Pointer
		len int
	}
	h := (*strHdr)(unsafe.Pointer(&tokens[0]))
	return *(*string)(unsafe.Pointer(&strHdr{ptr: h.ptr, len: total}))
}

// SpansToString reconstructs the full line text from spans.
func SpansToString(spans []IntraSpan) string {
	if len(spans) == 0 {
		return ""
	}
	total := 0
	for _, s := range spans {
		total += len(s.Text)
	}
	buf := make([]byte, 0, total)
	for _, s := range spans {
		buf = append(buf, s.Text...)
	}
	return string(buf)
}
