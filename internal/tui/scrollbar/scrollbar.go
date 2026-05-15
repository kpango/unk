// Package scrollbar provides pure scrollbar geometry computation for the unk TUI.
// All functions are stateless and take explicit arguments so they can be tested
// without constructing a model.
package scrollbar

// Geom holds the computed geometry for a proportional vertical scrollbar.
type Geom struct {
	ThumbTop  int // row index (0-based in body) where the thumb starts
	ThumbH    int // height of the thumb in rows
	MaxScroll int // maximum scrollTop value
	TrackLen  int // total track length (= bodyHeight)
}

// Compute returns the scrollbar geometry for a viewport of bodyH rows showing
// total content lines scrolled to scrollTop. Returns (zero, false) when the
// content fits entirely in the viewport (no scrollbar needed).
func Compute(scrollTop, total, bodyH int) (Geom, bool) {
	if total <= bodyH || bodyH <= 0 {
		return Geom{}, false
	}
	thumbH := max(1, bodyH*bodyH/total)
	maxScroll := total - bodyH
	thumbTop := 0
	if maxScroll > 0 {
		thumbTop = min(scrollTop*(bodyH-thumbH)/maxScroll, bodyH-thumbH)
	}
	return Geom{
		ThumbTop:  thumbTop,
		ThumbH:    thumbH,
		MaxScroll: maxScroll,
		TrackLen:  bodyH,
	}, true
}
