package render

import (
	"bytes"

	"github.com/charmbracelet/lipgloss"
	"github.com/kpango/unk/internal/highlight"
	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/tui/patch"
	"github.com/kpango/unk/internal/tui/styles"
	"github.com/kpango/unk/internal/tui/textutil"
	"github.com/kpango/unk/internal/types"
)

// SplitConfig contains all per-render configuration that SplitRenderer reads
// from the model. Callers build a SplitConfig from model fields and pass it to
// SplitRenderer.Init, keeping the render package free of model imports.
type SplitConfig struct {
	RS               *styles.RendererStyles
	Palette          styles.Palette
	DiffContentWidth int
	ShowLineNumbers  bool
	ThemeID          string
	CodeHOffset      int
	ShowUnkHeaders   bool
	ShowAgentNotes   bool
}

// SplitRenderer holds all per-render state for the split-view diff renderer.
// Storing state in a struct (rather than closures) lets the sequential render
// path keep the struct on the stack — zero heap allocations for 1–3 unks.
// The parallel path allocates once (heap *SplitRenderer) and passes r.RenderUnkInto
// as a method value to background goroutines.
type SplitRenderer struct {
	rs             *styles.RendererStyles
	hlStyle        highlight.Style
	hlCacheCtx     highlight.Cache
	hlCacheAdd     highlight.Cache
	hlCacheDel     highlight.Cache
	unks           []types.DiffUnk
	annotations    []types.AgentAnnotation
	divStr         string
	lang           string
	bgCtx          lipgloss.Color
	bgAdd          lipgloss.Color
	bgDel          lipgloss.Color
	leftContentW   int
	rightContentW  int
	lineNumWidth   int
	diffContentW   int
	codeHOffset    int
	showUnkHeaders bool
	showAgentNotes bool
	// Pre-built linenum template: open sequence + digit placeholder + close sequence.
	// writeLinenumBatch updates only the digit bytes in lineBuf, then does a single
	// dest.Write — no per-call copy of open/close. lineLen==0: lineNumWidth<=0 or overflow.
	lineBuf [64]byte
	lineOff int // byte offset of the digit area within lineBuf
	lineLen int // total length of the linenum template
	// emptyLineBuf is the linenum template with spaces in the digit area (never modified).
	// writeEmptyLinenumBatch writes emptyLineBuf[:lineLen] in one Write call.
	emptyLineBuf [64]byte
	// emptyLeftCodeBuf / emptyRightCodeBuf hold the pre-built empty code areas for
	// left/right sides: ctx-open + codeW spaces + ctx-close. A single Write replaces
	// the previous open+spaces+close triple. Len==0: codeW==0 or overflow (fallback).
	emptyLeftCodeBuf  [256]byte
	emptyLeftCodeLen  int
	emptyRightCodeBuf [256]byte
	emptyRightCodeLen int
	// Per-background inline highlight caches. Each render starts fresh (SplitRenderer
	// is stack-allocated for sequential renders). They act as per-render L1 caches
	// in front of the global highlight.TextLineCache, eliminating the RWMutex+hash
	// overhead for repeated identical lines (common in code diffs).
	// IMPORTANT: these are goroutine-local — the parallel path (renderSplitParallel)
	// sets noIC=true so writeHlCell passes nil and never touches these fields.
	noIC  bool
	icCtx textutil.InlineHlCache
	icDel textutil.InlineHlCache
	icAdd textutil.InlineHlCache
}

// Init initialises the SplitRenderer from a SplitConfig, DiffFile, and patch lines.
func (r *SplitRenderer) Init(cfg SplitConfig, f types.DiffFile, lines []string) {
	rs := cfg.RS
	p := cfg.Palette
	halfW := (cfg.DiffContentWidth - layout.BoxCharW) / 2
	r.rs = rs
	r.divStr = rs.DivStr
	r.leftContentW = max(halfW, 4)
	r.rightContentW = max(cfg.DiffContentWidth-halfW-layout.BoxCharW, 4)
	r.diffContentW = cfg.DiffContentWidth
	r.lineNumWidth = 0
	if cfg.ShowLineNumbers {
		r.lineNumWidth = max(textutil.ComputeLineNumWidth(lines), 4)
	}
	r.lang = ""
	if f.Language != nil {
		r.lang = *f.Language
	}
	r.hlStyle = highlight.ForTheme(cfg.ThemeID)
	r.unks = f.Metadata.Unks
	r.bgCtx = lipgloss.Color(p.ContextBg)
	r.bgAdd = lipgloss.Color(p.AddedBg)
	r.bgDel = lipgloss.Color(p.RemovedBg)
	r.codeHOffset = cfg.CodeHOffset
	r.showUnkHeaders = cfg.ShowUnkHeaders
	r.showAgentNotes = cfg.ShowAgentNotes
	if r.lang != "" {
		r.hlCacheCtx = r.hlStyle.CacheFor(r.lang, r.bgCtx)
		r.hlCacheAdd = r.hlStyle.CacheFor(r.lang, r.bgAdd)
		r.hlCacheDel = r.hlStyle.CacheFor(r.lang, r.bgDel)
	}
	r.annotations = r.annotations[:0]
	if cfg.ShowAgentNotes && f.Agent != nil {
		r.annotations = append(r.annotations, f.Agent.Annotations...)
	}

	// Pre-build rendering templates. All operations write into fixed-size struct
	// fields (stack storage) — zero heap allocations.

	// lineBuf: open sequence + digit placeholder + close sequence.
	// emptyLineBuf: same layout but with spaces permanently in the digit area.
	if r.lineNumWidth > 0 {
		lnOpen := r.rs.RawLineNum.Open
		lnClose := r.rs.RawLineNum.Close
		oLen := len(lnOpen)
		cLen := len(lnClose)
		total := oLen + r.lineNumWidth + cLen
		if total <= 64 {
			copy(r.lineBuf[:oLen], lnOpen)
			r.lineOff = oLen
			r.lineLen = total
			for i := range r.lineNumWidth {
				r.lineBuf[oLen+i] = ' '
			}
			copy(r.lineBuf[oLen+r.lineNumWidth:total], lnClose)
			// emptyLineBuf is a snapshot of lineBuf at this moment (all spaces in digit area).
			// writeLinenumBatch modifies lineBuf but never touches emptyLineBuf.
			copy(r.emptyLineBuf[:total], r.lineBuf[:total])
		}
	}

	// emptyLeftCodeBuf / emptyRightCodeBuf: ctx-open + codeW spaces + ctx-close.
	{
		ctxOpen := r.rs.RawDiffCtx.Open
		ctxClose := r.rs.RawDiffCtx.Close
		oLen := len(ctxOpen)
		cLen := len(ctxClose)

		leftCW := max(r.leftContentW-r.lineNumWidth, 0)
		if leftCW > 0 {
			total := oLen + leftCW + cLen
			if total <= len(r.emptyLeftCodeBuf) {
				copy(r.emptyLeftCodeBuf[:oLen], ctxOpen)
				copy(r.emptyLeftCodeBuf[oLen:oLen+leftCW], textutil.SpaceBuf[:leftCW])
				copy(r.emptyLeftCodeBuf[oLen+leftCW:total], ctxClose)
				r.emptyLeftCodeLen = total
			}
		}

		rightCW := max(r.rightContentW-r.lineNumWidth, 0)
		if rightCW == leftCW {
			// Symmetric: share the same content (copy from left buffer).
			copy(r.emptyRightCodeBuf[:r.emptyLeftCodeLen], r.emptyLeftCodeBuf[:r.emptyLeftCodeLen])
			r.emptyRightCodeLen = r.emptyLeftCodeLen
		} else if rightCW > 0 {
			total := oLen + rightCW + cLen
			if total <= len(r.emptyRightCodeBuf) {
				copy(r.emptyRightCodeBuf[:oLen], ctxOpen)
				copy(r.emptyRightCodeBuf[oLen:oLen+rightCW], textutil.SpaceBuf[:rightCW])
				copy(r.emptyRightCodeBuf[oLen+rightCW:total], ctxClose)
				r.emptyRightCodeLen = total
			}
		}
	}
}

// writeLinenumBatch writes the linenum cell prefix (ANSI open + right-justified
// number + ANSI close) as a SINGLE dest.Write call. The open/close sequences are
// pre-copied into r.lineBuf by Init(); only the digit bytes change per call.
// Eliminates 2 memmoves (copy of open + close) that the old stack-buf approach paid
// on every call. lineLen==0 means lineNumWidth<=0 or template overflows 64 bytes.
//
// Sequential path (r.noIC==false): modifies r.lineBuf in place (goroutine-local).
// Parallel path (r.noIC==true): r is shared across goroutines; uses a local copy of
// the template so concurrent goroutines do not race on r.lineBuf writes.
func (r *SplitRenderer) writeLinenumBatch(dest *bytes.Buffer, lineNum int) {
	if r.lineLen == 0 {
		return
	}
	var localBuf [64]byte
	var buf []byte
	if r.noIC {
		// Parallel path: copy template locally to avoid data race on r.lineBuf.
		copy(localBuf[:r.lineLen], r.lineBuf[:r.lineLen])
		buf = localBuf[:]
	} else {
		buf = r.lineBuf[:]
	}
	n := r.lineOff
	if r.lineNumWidth == 4 {
		buf[n+3] = ' '
		switch {
		case lineNum < 10:
			buf[n] = ' '
			buf[n+1] = ' '
			buf[n+2] = byte('0' + lineNum)
		case lineNum < 100:
			buf[n] = ' '
			buf[n+1] = byte('0' + lineNum/10)
			buf[n+2] = byte('0' + lineNum%10)
		case lineNum < 1000:
			buf[n] = byte('0' + lineNum/100)
			buf[n+1] = byte('0' + (lineNum/10)%10)
			buf[n+2] = byte('0' + lineNum%10)
		default:
			buf[n] = byte('0' + (lineNum/1000)%10)
			buf[n+1] = byte('0' + (lineNum/100)%10)
			buf[n+2] = byte('0' + (lineNum/10)%10)
			buf[n+3] = byte('0' + lineNum%10)
		}
	} else {
		// General: fill digit area with spaces, then overwrite with right-justified number.
		w := r.lineNumWidth
		for i := range w {
			buf[n+i] = ' '
		}
		if lineNum > 0 {
			var tmp [20]byte
			ti := len(tmp)
			for v := lineNum; v > 0; v /= 10 {
				ti--
				tmp[ti] = byte('0' + v%10)
			}
			nDigits := len(tmp) - ti
			pos := n + w - 1 - nDigits // start of digit area (trailing space at n+w-1)
			copy(buf[pos:], tmp[ti:])
		}
	}
	dest.Write(buf[:r.lineLen])
}

// writeEmptyLinenumBatch writes the empty linenum prefix from emptyLineBuf in
// a single Write call. emptyLineBuf is initialized in Init() and never modified.
func (r *SplitRenderer) writeEmptyLinenumBatch(dest *bytes.Buffer) {
	if r.lineLen == 0 {
		return
	}
	dest.Write(r.emptyLineBuf[:r.lineLen])
}

func (r *SplitRenderer) writeEmptyCell(dest *bytes.Buffer, colW int) {
	r.writeEmptyLinenumBatch(dest)
	// Use pre-built code area buffer (1 Write) instead of open+spaces+close (3 writes).
	if colW == r.leftContentW {
		if l := r.emptyLeftCodeLen; l > 0 {
			dest.Write(r.emptyLeftCodeBuf[:l])
			return
		}
	} else {
		if l := r.emptyRightCodeLen; l > 0 {
			dest.Write(r.emptyRightCodeBuf[:l])
			return
		}
	}
	// Fallback (buffer too small or codeW==0).
	if codeW := max(colW-r.lineNumWidth, 0); codeW > 0 {
		r.rs.RawDiffCtx.WriteOpen(dest)
		textutil.WriteSpaces(dest, codeW)
		r.rs.RawDiffCtx.WriteClose(dest)
	}
}

// writeCtxLineSymmetric renders one context line when leftContentW == rightContentW.
// It performs the IC lookup once and reuses the prebuilt cell for the right side,
// avoiding the second linear IC scan + string comparison that writeHlCell would pay.
// The output is: [left_linenum][cell][divLF_prefix][right_linenum][cell]\n
func (r *SplitRenderer) writeCtxLineSymmetric(dest *bytes.Buffer, text string, oldLine, newLine int, ic *textutil.InlineHlCache) {
	codeW := max(r.leftContentW-r.lineNumWidth, 0)
	// Cache hit: write prebuilt cell (hl+spaces+reset) for both sides in one WriteString each.
	if ic != nil && r.lang != "" {
		if cell, ok := ic.Get(text, codeW); ok {
			r.writeLinenumBatch(dest, oldLine)
			dest.WriteString(cell)
			dest.WriteString(r.divStr)
			r.writeLinenumBatch(dest, newLine)
			dest.WriteString(cell)
			dest.WriteByte('\n')
			return
		}
	}
	// Cache miss: compute hl, build cell, store for future hits.
	code := textutil.ApplyHorizontalOffset(text, r.codeHOffset)
	var codeVis int
	if codeW > 0 {
		code, codeVis = textutil.TruncateTabAware(code, codeW)
	}
	if r.lang != "" {
		var hl string
		var ok bool
		hl, ok = highlight.LineFromCache(code, r.hlCacheCtx)
		if !ok {
			hl = highlight.Line(code, r.lang, r.bgCtx, r.hlStyle)
		}
		pad := max(codeW-codeVis, 0)
		if hl != "" {
			if ic != nil {
				ic.Set(text, codeW, textutil.BuildCell(hl, pad))
			}
			// Left side.
			r.writeLinenumBatch(dest, oldLine)
			dest.WriteString(hl)
			if pad > 0 {
				textutil.WriteSpaces(dest, pad)
			}
			dest.WriteString("\x1b[m")
			dest.WriteString(r.divStr)
			// Right side: same output, different line number.
			r.writeLinenumBatch(dest, newLine)
			dest.WriteString(hl)
			if pad > 0 {
				textutil.WriteSpaces(dest, pad)
			}
			dest.WriteString("\x1b[m")
			dest.WriteByte('\n')
			return
		}
	} else {
		// Plain text (no highlighting): write both sides independently.
		r.writeLinenumBatch(dest, oldLine)
		r.rs.RawDiffCtx.WriteWidthTo(dest, textutil.ExpandTabs(code), codeW)
		dest.WriteString(r.divStr)
		r.writeLinenumBatch(dest, newLine)
		r.rs.RawDiffCtx.WriteWidthTo(dest, textutil.ExpandTabs(code), codeW)
		dest.WriteByte('\n')
		return
	}
	// Fallback: hl was empty.
	r.writeLinenumBatch(dest, oldLine)
	r.rs.RawDiffCtx.WriteWidthTo(dest, textutil.ExpandTabs(code), codeW)
	dest.WriteString(r.divStr)
	r.writeLinenumBatch(dest, newLine)
	r.rs.RawDiffCtx.WriteWidthTo(dest, textutil.ExpandTabs(code), codeW)
	dest.WriteByte('\n')
}

func (r *SplitRenderer) writeHlCell(dest *bytes.Buffer, text string, colW int, bg lipgloss.Color, rawBase styles.RawStyle, lineNum int, hlCache highlight.Cache, ic *textutil.InlineHlCache) {
	codeW := max(colW-r.lineNumWidth, 0)

	// Cache hit: write linenum + prebuilt cell (hl+spaces+reset) in one WriteString.
	if ic != nil && r.lang != "" {
		if cell, ok := ic.Get(text, codeW); ok {
			r.writeLinenumBatch(dest, lineNum)
			dest.WriteString(cell)
			return
		}
	}

	// Cache miss: compute hl, build and cache the cell, then write output.
	code := textutil.ApplyHorizontalOffset(text, r.codeHOffset)
	var codeVis int
	if codeW > 0 {
		code, codeVis = textutil.TruncateTabAware(code, codeW)
	}
	if r.lang != "" {
		var hl string
		var ok bool
		hl, ok = highlight.LineFromCache(code, hlCache)
		if !ok {
			hl = highlight.Line(code, r.lang, bg, r.hlStyle)
		}
		pad := max(codeW-codeVis, 0)
		if hl != "" {
			if ic != nil {
				ic.Set(text, codeW, textutil.BuildCell(hl, pad))
			}
			r.writeLinenumBatch(dest, lineNum)
			dest.WriteString(hl)
			if pad > 0 {
				textutil.WriteSpaces(dest, pad)
			}
			dest.WriteString("\x1b[m")
			return
		}
		// Fallback: hl empty (rare — lang set but highlight returned nothing).
		r.writeLinenumBatch(dest, lineNum)
		rawBase.WriteWidthTo(dest, textutil.ExpandTabs(code), codeW)
		return
	}
	// No syntax highlighting: write plain text and return early.
	r.writeLinenumBatch(dest, lineNum)
	rawBase.WriteWidthTo(dest, textutil.ExpandTabs(code), codeW)
}

// RenderUnkCore renders the lines in seg into dest.
func (r *SplitRenderer) RenderUnkCore(dest *bytes.Buffer, seg patch.Segment) {
	oldLine := seg.StartOld
	newLine := seg.StartNew
	unkIdx := seg.UnkIdx

	// Derive IC pointers once. noIC is true for parallel goroutines that share r;
	// in that case we pass nil to writeHlCell so it skips the inline cache entirely,
	// avoiding concurrent reads/writes to the shared icCtx/icDel/icAdd fields.
	var icCtx, icDel, icAdd *textutil.InlineHlCache
	if !r.noIC {
		icCtx = &r.icCtx
		icDel = &r.icDel
		icAdd = &r.icAdd
	}

	// Inline arrays back del/add slices to avoid heap allocation when ≤ 8 lines.
	var dArr [8]string
	var aArr [8]string
	var dnArr [8]int
	var anArr [8]int
	dels := dArr[:0]
	adds := aArr[:0]
	delNums := dnArr[:0]
	addNums := anArr[:0]

	i := 0
	for i < len(seg.Lines) {
		line := seg.Lines[i]
		if len(line) == 0 {
			r.writeEmptyCell(dest, r.leftContentW)
			dest.WriteString(r.divStr)
			r.writeEmptyCell(dest, r.rightContentW)
			dest.WriteByte('\n')
			i++
			continue
		}
		switch line[0] {
		case '@':
			if r.showUnkHeaders {
				r.rs.RawDiffHH.WriteWidthTo(dest, textutil.ExpandTabs(line), r.diffContentW)
				dest.WriteByte('\n')
			}
			if r.showAgentNotes && unkIdx >= 0 && unkIdx < len(r.unks) {
				h := r.unks[unkIdx]
				for _, ann := range r.annotations {
					if patch.AnnotationMatchesUnk(ann, h) {
						dest.WriteString(AgentNoteBox(ann, r.diffContentW, r.rs))
					}
				}
			}
			i++

		case '-', '+':
			dels = dels[:0]
			adds = adds[:0]
			delNums = delNums[:0]
			addNums = addNums[:0]
			for i < len(seg.Lines) && len(seg.Lines[i]) > 0 && seg.Lines[i][0] == '-' {
				delNums = append(delNums, oldLine+1)
				dels = append(dels, seg.Lines[i][1:])
				oldLine++
				i++
			}
			for i < len(seg.Lines) && len(seg.Lines[i]) > 0 && seg.Lines[i][0] == '+' {
				addNums = append(addNums, newLine+1)
				adds = append(adds, seg.Lines[i][1:])
				newLine++
				i++
			}
			maxPairs := max(len(dels), len(adds))
			for j := range maxPairs {
				if j < len(dels) {
					r.writeHlCell(dest, dels[j], r.leftContentW, r.bgDel, r.rs.RawDiffDel, delNums[j], r.hlCacheDel, icDel)
				} else {
					r.writeEmptyCell(dest, r.leftContentW)
				}
				dest.WriteString(r.divStr)
				if j < len(adds) {
					r.writeHlCell(dest, adds[j], r.rightContentW, r.bgAdd, r.rs.RawDiffAdd, addNums[j], r.hlCacheAdd, icAdd)
				} else {
					r.writeEmptyCell(dest, r.rightContentW)
				}
				dest.WriteByte('\n')
			}

		default:
			oldLine++
			newLine++
			ctx := line[1:]
			// Symmetric-layout fast path: left and right share the same codeW, so the
			// IC result (codeVis, hl) from the left call can be reused for the right,
			// eliminating the second IC linear scan + string comparison.
			if r.leftContentW == r.rightContentW {
				r.writeCtxLineSymmetric(dest, ctx, oldLine, newLine, icCtx)
			} else {
				r.writeHlCell(dest, ctx, r.leftContentW, r.bgCtx, r.rs.RawDiffCtx, oldLine, r.hlCacheCtx, icCtx)
				dest.WriteString(r.divStr)
				r.writeHlCell(dest, ctx, r.rightContentW, r.bgCtx, r.rs.RawDiffCtx, newLine, r.hlCacheCtx, icCtx)
				dest.WriteByte('\n')
			}
			i++
		}
	}
}

// SetNoIC controls the inline highlight cache. Set to true before passing r to
// parallel goroutines — goroutines share the SplitRenderer and must not race on
// the goroutine-local icCtx/icDel/icAdd fields.
func (r *SplitRenderer) SetNoIC(v bool) { r.noIC = v }

// RenderUnkInto wraps RenderUnkCore for the parallel path: renders seg directly
// into buf. Called as a method value passed to RenderUnksParallelInto goroutines.
func (r *SplitRenderer) RenderUnkInto(seg patch.Segment, buf *bytes.Buffer) {
	buf.Grow(len(seg.Lines) * ((r.leftContentW+r.rightContentW)*4 + len(r.divStr) + 20))
	r.RenderUnkCore(buf, seg)
}
