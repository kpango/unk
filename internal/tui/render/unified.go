package render

import (
	"bytes"

	"github.com/charmbracelet/lipgloss"
	"github.com/kpango/unk/internal/diff"
	"github.com/kpango/unk/internal/highlight"
	"github.com/kpango/unk/internal/tui/patch"
	"github.com/kpango/unk/internal/tui/styles"
	"github.com/kpango/unk/internal/tui/textutil"
	"github.com/kpango/unk/internal/types"
)

// UnifiedConfig contains all per-render configuration that UnifiedRenderer reads
// from the model. Callers build a UnifiedConfig and pass it to UnifiedRenderer.Init,
// keeping the render package free of model imports.
type UnifiedConfig struct {
	RS               *styles.RendererStyles
	Palette          styles.Palette
	DiffContentWidth int
	ShowLineNumbers  bool
	WrapLines        bool
	ThemeID          string
	CodeHOffset      int
	ShowUnkHeaders   bool
	ShowAgentNotes   bool
}

// UnifiedRenderer holds all per-render state for the unified diff renderer.
// Closures that previously captured model fields are replaced by unexported methods,
// allowing the sequential path to keep the struct on the stack — zero heap allocations
// for 1–3 unks. The parallel path passes r.RenderUnkInto as a method value.
type UnifiedRenderer struct {
	rs            *styles.RendererStyles
	hlStyle       highlight.Style
	hlCacheCtx    highlight.Cache
	hlCacheAdd    highlight.Cache
	hlCacheDel    highlight.Cache
	unks          []types.DiffUnk
	annotations   []types.AgentAnnotation
	intraDelSpans [][]diff.IntraSpan
	intraAddSpans [][]diff.IntraSpan
	lang          string
	bgCtx         lipgloss.Color
	bgAdd         lipgloss.Color
	bgDel         lipgloss.Color
	lineNumWidth  int
	contentWidth  int
	codeWidth     int
	diffContentW  int
	codeHOffset   int
	wrapLines     bool
	showLineNums  bool
	showUnkHdrs   bool
	showAgentNotes bool
}

// Init initialises the UnifiedRenderer from a UnifiedConfig, DiffFile, patch lines,
// and the pre-fetched intra-span entry for this file.
func (r *UnifiedRenderer) Init(cfg UnifiedConfig, f types.DiffFile, lines []string, intraEntry [2][][]diff.IntraSpan) {
	p := cfg.Palette
	r.rs = cfg.RS
	r.diffContentW = cfg.DiffContentWidth
	r.wrapLines = cfg.WrapLines
	r.codeHOffset = cfg.CodeHOffset
	r.showLineNums = cfg.ShowLineNumbers
	r.showUnkHdrs = cfg.ShowUnkHeaders
	r.showAgentNotes = cfg.ShowAgentNotes

	r.lang = ""
	if f.Language != nil {
		r.lang = *f.Language
	}

	r.lineNumWidth = 0
	if cfg.ShowLineNumbers {
		r.lineNumWidth = textutil.ComputeLineNumWidth(lines)
	}
	r.contentWidth = cfg.DiffContentWidth - r.lineNumWidth
	r.codeWidth = r.contentWidth - 1

	r.hlStyle = highlight.ForTheme(cfg.ThemeID)
	r.bgCtx = lipgloss.Color(p.ContextBg)
	r.bgAdd = lipgloss.Color(p.AddedBg)
	r.bgDel = lipgloss.Color(p.RemovedBg)
	if r.lang != "" {
		r.hlCacheCtx = r.hlStyle.CacheFor(r.lang, r.bgCtx)
		r.hlCacheAdd = r.hlStyle.CacheFor(r.lang, r.bgAdd)
		r.hlCacheDel = r.hlStyle.CacheFor(r.lang, r.bgDel)
	}

	r.unks = f.Metadata.Unks
	r.intraDelSpans = intraEntry[0]
	r.intraAddSpans = intraEntry[1]

	r.annotations = r.annotations[:0]
	if cfg.ShowAgentNotes && f.Agent != nil {
		r.annotations = append(r.annotations, f.Agent.Annotations...)
	}
}

func (r *UnifiedRenderer) writeLineNum(dest *bytes.Buffer, n int, isUnkHeader bool) {
	if !r.showLineNums {
		return
	}
	r.rs.RawLineNum.WriteOpen(dest)
	if isUnkHeader {
		textutil.WriteSpaces(dest, r.lineNumWidth)
	} else {
		textutil.WriteRightJustifiedInt(dest, n, r.lineNumWidth)
	}
	r.rs.RawLineNum.WriteClose(dest)
}

// writeHL writes prefix + syntax-highlighted code + padding into dest.
// Returns false when no highlight is available (caller should fall back).
func (r *UnifiedRenderer) writeHL(dest *bytes.Buffer, prefix, codeText string, bg lipgloss.Color, rawBase styles.RawStyle, hlCache highlight.Cache) bool {
	if r.lang == "" {
		return false
	}
	codeText = textutil.ExpandTabs(codeText)
	var visW int
	if !r.wrapLines && r.codeWidth > 0 {
		codeText, visW = textutil.TruncateTabAware(codeText, r.codeWidth)
	} else {
		visW = textutil.VisibleWidth(codeText)
	}
	hl, ok := highlight.LineFromCache(codeText, hlCache)
	if !ok {
		hl = highlight.Line(codeText, r.lang, bg, r.hlStyle)
	}
	if hl == "" {
		return false
	}
	rawBase.WriteTo(dest, prefix)
	dest.WriteString(hl)
	if !r.wrapLines && r.codeWidth > 0 {
		if pad := max(r.codeWidth-visW, 0); pad > 0 {
			textutil.WriteSpaces(dest, pad)
		}
	}
	dest.WriteString("\x1b[m")
	return true
}

func (r *UnifiedRenderer) writeDiffLine(dest *bytes.Buffer, raw styles.RawStyle, marker byte, codeText string) {
	codeText = textutil.ExpandTabs(codeText)
	var visW int
	if !r.wrapLines && r.codeWidth > 0 {
		codeText, visW = textutil.TruncateTabAware(codeText, r.codeWidth)
	}
	raw.WriteOpen(dest)
	dest.WriteByte(marker)
	dest.WriteString(codeText)
	if !r.wrapLines && r.contentWidth > 1 {
		textutil.WriteSpaces(dest, r.contentWidth-1-visW)
	}
	raw.WriteClose(dest)
}

func (r *UnifiedRenderer) writeWithIntraSpans(dest *bytes.Buffer, marker byte, spans []diff.IntraSpan, rawBase, rawChanged styles.RawStyle) {
	var markerStr string
	switch marker {
	case '+':
		markerStr = "+"
	case '-':
		markerStr = "-"
	default:
		markerStr = " "
	}
	rawBase.WriteTo(dest, markerStr)
	remaining := r.codeWidth
	for _, span := range spans {
		if remaining <= 0 {
			break
		}
		text := textutil.ExpandTabs(span.Text)
		if !r.wrapLines && r.codeWidth > 0 {
			text = textutil.TruncateColumns(text, remaining)
			remaining -= textutil.VisibleWidth(text)
		}
		if span.Changed {
			rawChanged.WriteTo(dest, text)
		} else {
			rawBase.WriteTo(dest, text)
		}
	}
	if !r.wrapLines && remaining > 0 {
		rawBase.WriteOpen(dest)
		textutil.WriteSpaces(dest, remaining)
		rawBase.WriteClose(dest)
	}
}

// RenderUnkCore renders the lines in seg into dest.
func (r *UnifiedRenderer) RenderUnkCore(dest *bytes.Buffer, seg patch.Segment) {
	rs := r.rs
	oldLine := seg.StartOld
	newLine := seg.StartNew
	unkIdx := seg.UnkIdx

	for i, line := range seg.Lines {
		if len(line) == 0 {
			if r.showLineNums {
				rs.RawLineNum.WriteOpen(dest)
				textutil.WriteSpaces(dest, r.lineNumWidth)
				rs.RawLineNum.WriteClose(dest)
			}
			rs.RawDiffCtx.WriteWidthTo(dest, "", r.contentWidth)
			dest.WriteByte('\n')
			continue
		}
		li := seg.FirstLineIdx + i

		content := line
		if r.codeHOffset > 0 && len(line) > 1 {
			content = string(line[0]) + textutil.ApplyHorizontalOffset(line[1:], r.codeHOffset)
		}

		switch line[0] {
		case '+':
			r.writeLineNum(dest, newLine+1, false)
			if li < len(r.intraAddSpans) && r.intraAddSpans[li] != nil && r.codeHOffset == 0 {
				r.writeWithIntraSpans(dest, '+', r.intraAddSpans[li], rs.RawDiffAdd, rs.RawIntraAdd)
			} else {
				codeText := content[1:]
				if !r.writeHL(dest, "+", codeText, r.bgAdd, rs.RawDiffAdd, r.hlCacheAdd) {
					r.writeDiffLine(dest, rs.RawDiffAdd, '+', codeText)
				}
			}
			newLine++
		case '-':
			r.writeLineNum(dest, oldLine+1, false)
			if li < len(r.intraDelSpans) && r.intraDelSpans[li] != nil && r.codeHOffset == 0 {
				r.writeWithIntraSpans(dest, '-', r.intraDelSpans[li], rs.RawDiffDel, rs.RawIntraDel)
			} else {
				codeText := content[1:]
				if !r.writeHL(dest, "-", codeText, r.bgDel, rs.RawDiffDel, r.hlCacheDel) {
					r.writeDiffLine(dest, rs.RawDiffDel, '-', codeText)
				}
			}
			oldLine++
		case '@':
			unkIdx = seg.UnkIdx
			if r.showUnkHdrs {
				r.writeLineNum(dest, 0, true)
				if r.wrapLines {
					rs.RawDiffHH.WriteTo(dest, textutil.ExpandTabs(content))
				} else {
					rs.RawDiffHH.WriteWidthTo(dest, textutil.ExpandTabs(content), r.contentWidth)
				}
			}
		default:
			r.writeLineNum(dest, newLine+1, false)
			codeText := content[1:]
			if !r.writeHL(dest, " ", codeText, r.bgCtx, rs.RawDiffCtx, r.hlCacheCtx) {
				r.writeDiffLine(dest, rs.RawDiffCtx, ' ', codeText)
			}
			oldLine++
			newLine++
		}
		dest.WriteByte('\n')

		if r.showAgentNotes && line[0] == '@' && unkIdx >= 0 && unkIdx < len(r.unks) {
			h := r.unks[unkIdx]
			for _, ann := range r.annotations {
				if patch.AnnotationMatchesUnk(ann, h) {
					dest.WriteString(AgentNoteBox(ann, r.diffContentW, rs))
				}
			}
		}
	}
}

// RenderUnkInto wraps RenderUnkCore for the parallel path: renders seg directly
// into buf. Called as a method value passed to RenderUnksParallelInto goroutines.
func (r *UnifiedRenderer) RenderUnkInto(seg patch.Segment, buf *bytes.Buffer) {
	buf.Grow(len(seg.Lines) * (r.contentWidth*4 + r.lineNumWidth + 20))
	r.RenderUnkCore(buf, seg)
}
