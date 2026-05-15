package render

import (
	"bytes"
	"sync"

	"github.com/kpango/unk/internal/tui/patch"
	"github.com/kpango/unk/internal/tui/styles"
	"github.com/kpango/unk/internal/tui/textutil"
	"github.com/kpango/unk/internal/types"
)

// stackRendererPool pools *StackRenderer objects so that the backing slices for
// blocks/allCtx/allDels/allAdds survive across renders, eliminating the four
// make() calls (and ColFill sep recomputation) that Init would otherwise pay.
var stackRendererPool = sync.Pool{New: func() any { return &StackRenderer{} }}

// AcquireStackRenderer returns a pooled *StackRenderer ready for Init.
func AcquireStackRenderer() *StackRenderer {
	return stackRendererPool.Get().(*StackRenderer)
}

// ReleaseStackRenderer returns r to the pool. Callers must not use r after this.
func ReleaseStackRenderer(r *StackRenderer) {
	stackRendererPool.Put(r)
}

// StackConfig contains all per-render configuration for the stack diff renderer.
type StackConfig struct {
	RS               *styles.RendererStyles
	DiffContentWidth int
	ShowLineNumbers  bool
	WrapLines        bool
	CodeHOffset      int
	ShowUnkHeaders   bool
	ShowAgentNotes   bool
}

// lineEntry holds one rendered line for the stack view.
// text is raw content without the leading prefix char (tabs unexpanded).
type lineEntry struct {
	num    int
	prefix byte
	text   string
}

// unkBlock references line ranges in the shared flat slices allCtx/allDels/allAdds.
type unkBlock struct {
	header          string
	unkIdx          int
	ctxBase, ctxLen int
	delBase, delLen int
	addBase, addLen int
}

// StackRenderer parses patch lines and renders the stacked diff view.
// Init performs the parse phase; RenderInto performs the render phase.
// Storing state in a struct (rather than closures) eliminates per-closure heap allocs.
// When obtained via AcquireStackRenderer, the slice fields and sep cache survive
// across renders, eliminating repeated make() and ColFill calls.
type StackRenderer struct {
	rs             *styles.RendererStyles
	annotations    []types.AgentAnnotation
	unks           []types.DiffUnk
	sep            string
	sepWidth       int // DiffContentWidth for which sep was last computed
	diffContentW   int
	contentWidth   int
	lineNumWidth   int
	wrapLines      bool
	showLineNums   bool
	showUnkHdrs    bool
	showAgentNotes bool
	blocks         []unkBlock
	allCtx         []lineEntry
	allDels        []lineEntry
	allAdds        []lineEntry
	// Pre-built line-number template (same as split renderer's lineBuf).
	// writeLineNumBatch fills only the digit bytes and writes the whole buffer in one
	// dest.Write call, replacing the previous 3-write WriteOpen/WriteInt/WriteClose.
	// lineLen==0 when ShowLineNumbers is false or the template overflows 64 bytes.
	lineBuf     [64]byte
	lineOff     int // byte offset of digit area within lineBuf
	lineLen     int // total length of lineBuf template
	emptyLineBuf [64]byte // same layout but digit area is spaces (for empty-line rows)
}

// Init configures the StackRenderer and parses lines into internal block state.
func (r *StackRenderer) Init(cfg StackConfig, f types.DiffFile, lines []string) {
	r.rs = cfg.RS
	r.diffContentW = cfg.DiffContentWidth
	r.wrapLines = cfg.WrapLines
	r.showLineNums = cfg.ShowLineNumbers
	r.showUnkHdrs = cfg.ShowUnkHeaders
	r.showAgentNotes = cfg.ShowAgentNotes

	r.lineNumWidth = 0
	if cfg.ShowLineNumbers {
		r.lineNumWidth = textutil.ComputeLineNumWidth(lines)
	}
	r.contentWidth = cfg.DiffContentWidth - r.lineNumWidth

	r.unks = f.Metadata.Unks
	r.annotations = r.annotations[:0]
	if cfg.ShowAgentNotes && f.Agent != nil {
		r.annotations = append(r.annotations, f.Agent.Annotations...)
	}

	// Reuse cached sep when DiffContentWidth hasn't changed (common case in a session).
	if r.sepWidth != cfg.DiffContentWidth {
		r.sep = textutil.ColFill('─', cfg.DiffContentWidth)
		r.sepWidth = cfg.DiffContentWidth
	}

	// Build line-number template: ANSI-open + digit-placeholder + ANSI-close.
	// writeLineNumBatch modifies only the digit bytes and writes the whole template in
	// one dest.Write call, replacing the 3-write WriteOpen/WriteInt/WriteClose sequence.
	r.lineLen = 0
	if r.lineNumWidth > 0 {
		lnOpen := r.rs.RawLineNum.Open
		lnClose := r.rs.RawLineNum.Close
		total := len(lnOpen) + r.lineNumWidth + len(lnClose)
		if total <= 64 {
			copy(r.lineBuf[:len(lnOpen)], lnOpen)
			r.lineOff = len(lnOpen)
			for i := range r.lineNumWidth {
				r.lineBuf[r.lineOff+i] = ' '
			}
			copy(r.lineBuf[r.lineOff+r.lineNumWidth:total], lnClose)
			copy(r.emptyLineBuf[:total], r.lineBuf[:total])
			r.lineLen = total
		}
	}

	// Parse lines into blocks/allCtx/allDels/allAdds.
	nUnks := len(f.Metadata.Unks)
	if nUnks == 0 {
		nUnks = 4
	}
	n := len(lines)
	if cap(r.blocks) >= nUnks {
		r.blocks = r.blocks[:0]
	} else {
		r.blocks = make([]unkBlock, 0, nUnks)
	}
	if cap(r.allCtx) >= n/2+1 {
		r.allCtx = r.allCtx[:0]
	} else {
		r.allCtx = make([]lineEntry, 0, n/2+1)
	}
	if cap(r.allDels) >= n/4+1 {
		r.allDels = r.allDels[:0]
	} else {
		r.allDels = make([]lineEntry, 0, n/4+1)
	}
	if cap(r.allAdds) >= n/4+1 {
		r.allAdds = r.allAdds[:0]
	} else {
		r.allAdds = make([]lineEntry, 0, n/4+1)
	}

	oldLine, newLine := 0, 0
	unkIdx := -1
	curIdx := -1

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case '@':
			if curIdx >= 0 {
				r.blocks[curIdx].ctxLen = len(r.allCtx) - r.blocks[curIdx].ctxBase
				r.blocks[curIdx].delLen = len(r.allDels) - r.blocks[curIdx].delBase
				r.blocks[curIdx].addLen = len(r.allAdds) - r.blocks[curIdx].addBase
			}
			o, nv := patch.ParseUnkHeader(line)
			if o >= 0 {
				oldLine = o - 1
			}
			if nv >= 0 {
				newLine = nv - 1
			}
			unkIdx++
			r.blocks = append(r.blocks, unkBlock{
				header:  line,
				unkIdx:  unkIdx,
				ctxBase: len(r.allCtx),
				delBase: len(r.allDels),
				addBase: len(r.allAdds),
			})
			curIdx = len(r.blocks) - 1
		case '+':
			if curIdx < 0 {
				break
			}
			newLine++
			content := line[1:]
			if cfg.CodeHOffset > 0 {
				content = textutil.ApplyHorizontalOffset(content, cfg.CodeHOffset)
			}
			r.allAdds = append(r.allAdds, lineEntry{newLine, '+', content})
		case '-':
			if curIdx < 0 {
				break
			}
			oldLine++
			content := line[1:]
			if cfg.CodeHOffset > 0 {
				content = textutil.ApplyHorizontalOffset(content, cfg.CodeHOffset)
			}
			r.allDels = append(r.allDels, lineEntry{oldLine, '-', content})
		default:
			if curIdx < 0 {
				break
			}
			oldLine++
			newLine++
			content := line[1:]
			if cfg.CodeHOffset > 0 {
				content = textutil.ApplyHorizontalOffset(content, cfg.CodeHOffset)
			}
			r.allCtx = append(r.allCtx, lineEntry{newLine, ' ', content})
		}
	}
	if curIdx >= 0 {
		r.blocks[curIdx].ctxLen = len(r.allCtx) - r.blocks[curIdx].ctxBase
		r.blocks[curIdx].delLen = len(r.allDels) - r.blocks[curIdx].delBase
		r.blocks[curIdx].addLen = len(r.allAdds) - r.blocks[curIdx].addBase
	}
}

// writeLineNumBatch writes the line-number prefix in a single dest.Write call using the
// pre-built template. Falls back to the 3-write path only if the template overflowed 64 bytes.
func (r *StackRenderer) writeLineNumBatch(dest *bytes.Buffer, n int) {
	if !r.showLineNums {
		return
	}
	if r.lineLen == 0 {
		// Template too large; fall back to the original 3-write approach.
		r.rs.RawLineNum.WriteOpen(dest)
		textutil.WriteRightJustifiedInt(dest, n, r.lineNumWidth)
		r.rs.RawLineNum.WriteClose(dest)
		return
	}
	// Stamp digit bytes into the template (stack-local, safe with pooled renderer).
	off := r.lineOff
	w := r.lineNumWidth
	if n <= 0 {
		dest.Write(r.emptyLineBuf[:r.lineLen])
		return
	}
	if w == 4 {
		r.lineBuf[off+3] = ' '
		switch {
		case n < 10:
			r.lineBuf[off] = ' '; r.lineBuf[off+1] = ' '; r.lineBuf[off+2] = byte('0' + n)
		case n < 100:
			r.lineBuf[off] = ' '; r.lineBuf[off+1] = byte('0' + n/10); r.lineBuf[off+2] = byte('0' + n%10)
		case n < 1000:
			r.lineBuf[off] = byte('0' + n/100); r.lineBuf[off+1] = byte('0' + (n/10)%10); r.lineBuf[off+2] = byte('0' + n%10)
		default:
			r.lineBuf[off] = byte('0' + (n/1000)%10); r.lineBuf[off+1] = byte('0' + (n/100)%10)
			r.lineBuf[off+2] = byte('0' + (n/10)%10); r.lineBuf[off+3] = byte('0' + n%10)
		}
	} else {
		for i := range w {
			r.lineBuf[off+i] = ' '
		}
		var tmp [20]byte
		ti := len(tmp)
		for v := n; v > 0; v /= 10 {
			ti--
			tmp[ti] = byte('0' + v%10)
		}
		nDigits := len(tmp) - ti
		pos := off + w - 1 - nDigits
		copy(r.lineBuf[pos:], tmp[ti:])
	}
	dest.Write(r.lineBuf[:r.lineLen])
}

// RenderInto writes the stacked diff output into dest.
// Pre-grows the buffer to avoid repeated grow-and-copy cycles.
func (r *StackRenderer) RenderInto(dest *bytes.Buffer) {
	// Estimate output size: lines + unk headers + del/add separators.
	lineCount := len(r.allCtx) + len(r.allDels) + len(r.allAdds)
	if lineCount > 0 {
		nBlocks := len(r.blocks)
		dest.Grow(lineCount*(r.lineNumWidth+r.contentWidth+35) +
			nBlocks*(r.diffContentW*3+55) + // unk header lines
			nBlocks*(len(r.sep)+10))        // del/add separator lines
	}
	rs := r.rs
	hhStyle := rs.DiffHH

	for _, blk := range r.blocks {
		if r.showUnkHdrs {
			if !r.wrapLines {
				rs.RawDiffHH.WriteWidthTo(dest, textutil.ExpandTabs(blk.header), r.diffContentW)
			} else {
				dest.WriteString(hhStyle.Render(textutil.ExpandTabs(blk.header)))
			}
			dest.WriteByte('\n')
		}
		if r.showAgentNotes && blk.unkIdx >= 0 && blk.unkIdx < len(r.unks) {
			h := r.unks[blk.unkIdx]
			for _, ann := range r.annotations {
				if patch.AnnotationMatchesUnk(ann, h) {
					dest.WriteString(AgentNoteBox(ann, r.diffContentW, rs))
				}
			}
		}
		for _, l := range r.allCtx[blk.ctxBase : blk.ctxBase+blk.ctxLen] {
			r.writeLineNumBatch(dest, l.num)
			rs.RawDiffCtx.WritePrefixedRawWidthTo(dest, l.prefix, l.text, r.contentWidth)
			dest.WriteByte('\n')
		}
		for _, l := range r.allDels[blk.delBase : blk.delBase+blk.delLen] {
			r.writeLineNumBatch(dest, l.num)
			rs.RawDiffDel.WritePrefixedRawWidthTo(dest, l.prefix, l.text, r.contentWidth)
			dest.WriteByte('\n')
		}
		if blk.delLen > 0 && blk.addLen > 0 {
			rs.RawBorder.WriteTo(dest, r.sep)
			dest.WriteByte('\n')
		}
		for _, l := range r.allAdds[blk.addBase : blk.addBase+blk.addLen] {
			r.writeLineNumBatch(dest, l.num)
			rs.RawDiffAdd.WritePrefixedRawWidthTo(dest, l.prefix, l.text, r.contentWidth)
			dest.WriteByte('\n')
		}
	}
}
