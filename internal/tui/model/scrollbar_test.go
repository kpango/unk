package model

import (
	"testing"

	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/types"
)

// makeScrollModel builds a minimal Model with enough diff lines and a fixed
// terminal size to exercise scrollbar geometry and click/drag math.
func makeScrollModel(termH, totalLines int) *model {
	// Produce enough synthetic unks to give totalLines diff lines.
	unksNeeded := (totalLines + 9) / 10
	unkMeta := make([]types.DiffUnk, unksNeeded)
	for i := range unkMeta {
		unkMeta[i] = types.DiffUnk{Index: i, Header: "@@ -1,10 +1,10 @@"}
	}
	patch := ""
	for range totalLines {
		patch += "+line\n"
	}
	cs := types.Changeset{Files: []types.DiffFile{{
		ID:       "f",
		Path:     "f.go",
		Patch:    patch,
		Metadata: types.DiffMetadata{CacheKey: "ck", Unks: unkMeta},
	}}}
	m := New(types.Bootstrap{Changeset: cs}).(*model)
	m.termWidth = 80
	m.termHeight = termH
	m.layout = layout.ComputeLayout(m.termWidth, m.termHeight, types.LayoutModeAuto, 34, false, false)
	m.isLoading = false
	return m
}

func TestScrollbarGeomNoScroll(t *testing.T) {
	// When content fits, scrollbarGeom returns false.
	m := makeScrollModel(40, 10)
	_, ok := m.scrollbarGeom()
	if ok {
		t.Fatal("expected no scrollbar when content fits in viewport")
	}
}

func TestScrollbarGeomBasic(t *testing.T) {
	m := makeScrollModel(20, 200) // bodyH≈17, total=200
	g, ok := m.scrollbarGeom()
	if !ok {
		t.Fatal("expected scrollbar for content larger than viewport")
	}
	if g.ThumbH < 1 {
		t.Errorf("thumbH must be at least 1, got %d", g.ThumbH)
	}
	if g.ThumbTop < 0 || g.ThumbTop+g.ThumbH > g.TrackLen {
		t.Errorf("thumb out of track: thumbTop=%d thumbH=%d trackLen=%d", g.ThumbTop, g.ThumbH, g.TrackLen)
	}
	if g.MaxScroll <= 0 {
		t.Errorf("maxScroll must be positive, got %d", g.MaxScroll)
	}
}

func TestScrollbarGeomThumbAtBottom(t *testing.T) {
	m := makeScrollModel(20, 200)
	g, _ := m.scrollbarGeom()
	m.scrollTop = g.MaxScroll
	g2, _ := m.scrollbarGeom()
	if g2.ThumbTop+g2.ThumbH != g2.TrackLen {
		t.Errorf("at maxScroll, thumb should reach the bottom: thumbTop=%d thumbH=%d trackLen=%d",
			g2.ThumbTop, g2.ThumbH, g2.TrackLen)
	}
}

func TestHandleScrollbarPressTrack(t *testing.T) {
	m := makeScrollModel(20, 200)
	m.scrollTop = 0

	// Click somewhere in the track (not on the thumb at scrollTop=0, which is at the top).
	bodyH := m.bodyHeight()
	clickRow := bodyH / 2 // middle of track
	m.handleScrollbarPress(m.bodyStartRow() + clickRow)

	if m.scrollTop == 0 {
		t.Error("clicking mid-track should scroll away from 0")
	}
	g, _ := m.scrollbarGeom()
	if m.scrollTop < 0 || m.scrollTop > g.MaxScroll {
		t.Errorf("scrollTop %d out of [0, %d]", m.scrollTop, g.MaxScroll)
	}
}

func TestHandleScrollbarPressThumb(t *testing.T) {
	m := makeScrollModel(20, 200)
	m.scrollTop = 0

	g, _ := m.scrollbarGeom()
	// Click exactly on the thumb start.
	m.handleScrollbarPress(m.bodyStartRow() + g.ThumbTop)

	if !m.scrollbarDragging {
		t.Error("clicking on thumb should start dragging")
	}
	if m.scrollbarDragScrollTop != 0 {
		t.Errorf("drag anchor scrollTop should be 0, got %d", m.scrollbarDragScrollTop)
	}
}

func TestHandleScrollbarDrag(t *testing.T) {
	m := makeScrollModel(20, 200)
	m.scrollTop = 0

	g, _ := m.scrollbarGeom()
	anchorY := m.bodyStartRow() + g.ThumbTop
	m.scrollbarDragging = true
	m.scrollbarDragAnchorY = anchorY
	m.scrollbarDragScrollTop = 0

	// Drag 3 rows down.
	m.handleScrollbarDrag(anchorY + 3)

	if m.scrollTop <= 0 {
		t.Error("dragging down should increase scrollTop")
	}
	g2, _ := m.scrollbarGeom()
	if m.scrollTop > g2.MaxScroll {
		t.Errorf("scrollTop %d exceeds maxScroll %d", m.scrollTop, g2.MaxScroll)
	}
}

func TestScrollbarDragClampedToMax(t *testing.T) {
	m := makeScrollModel(20, 200)
	m.scrollTop = 0

	g, _ := m.scrollbarGeom()
	m.scrollbarDragging = true
	m.scrollbarDragAnchorY = m.bodyStartRow()
	m.scrollbarDragScrollTop = 0

	// Drag far below the bottom — scrollTop must be clamped.
	m.handleScrollbarDrag(m.bodyStartRow() + g.TrackLen + 999)

	if m.scrollTop != g.MaxScroll {
		t.Errorf("scrollTop should be clamped to maxScroll=%d, got %d", g.MaxScroll, m.scrollTop)
	}
}

func TestScrollbarDragClampedToZero(t *testing.T) {
	m := makeScrollModel(20, 200)
	g0, _ := m.scrollbarGeom()
	m.scrollTop = g0.MaxScroll / 2

	m.scrollbarDragging = true
	m.scrollbarDragAnchorY = m.bodyStartRow() + g0.TrackLen/2
	m.scrollbarDragScrollTop = m.scrollTop

	// Drag far above the top.
	m.handleScrollbarDrag(m.bodyStartRow() - 999)

	if m.scrollTop != 0 {
		t.Errorf("scrollTop should be clamped to 0, got %d", m.scrollTop)
	}
}
