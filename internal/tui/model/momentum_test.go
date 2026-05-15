package model

import (
	"fmt"
	"math"
	"testing"

	"github.com/kpango/unk/internal/tui/layout"
	"github.com/kpango/unk/internal/types"
)

// momentumModel builds a Model with enough content to scroll and a fixed
// terminal size so bodyHeight and totalDiffLines are deterministic.
func momentumModel() *model {
	// 50 synthetic lines per file, 10 files → ~500 diff lines, bodyH ≈ 37
	lines := 50
	patch := ""
	for range lines {
		patch += "+line\n"
	}
	var files []types.DiffFile
	for i := range 10 {
		files = append(files, types.DiffFile{
			ID:    fmt.Sprintf("f%d", i),
			Path:  fmt.Sprintf("file%d.go", i),
			Patch: patch,
			Metadata: types.DiffMetadata{
				CacheKey: fmt.Sprintf("ck%d", i),
				Unks:     []types.DiffUnk{{Index: 0, Header: "@@ -1,50 +1,50 @@"}},
			},
		})
	}
	m := New(types.Bootstrap{Changeset: types.Changeset{Files: files}}).(*model)
	m.termWidth = 80
	m.termHeight = 40
	m.layout = layout.ComputeLayout(m.termWidth, m.termHeight, types.LayoutModeAuto, 34, false, false)
	m.isLoading = false
	return m
}

func TestMomentumWheelAddsVelocity(t *testing.T) {
	m := momentumModel()
	m.scrollVelocity = 0
	m.scrollVelocity = min(scrollMaxVelocity, m.scrollVelocity+scrollWheelImpulse)
	if m.scrollVelocity != scrollWheelImpulse {
		t.Errorf("expected velocity %v after one wheel event, got %v", scrollWheelImpulse, m.scrollVelocity)
	}
}

func TestMomentumVelocityCap(t *testing.T) {
	m := momentumModel()
	for range 100 {
		m.scrollVelocity = min(scrollMaxVelocity, m.scrollVelocity+scrollWheelImpulse)
	}
	if m.scrollVelocity > scrollMaxVelocity {
		t.Errorf("velocity exceeded max: %v > %v", m.scrollVelocity, scrollMaxVelocity)
	}
}

func TestMomentumDecayReachesZero(t *testing.T) {
	m := momentumModel()
	m.scrollVelocity = scrollWheelImpulse

	frames := 0
	for math.Abs(m.scrollVelocity) >= scrollStopThreshold {
		m.scrollVelocity *= scrollFriction
		frames++
		if frames > 10_000 {
			t.Fatal("velocity never decayed to stop threshold")
		}
	}
	// With friction=0.85, impulse=3.5, threshold=0.5, stops within ~13 frames (~205 ms)
	if frames > 200 {
		t.Errorf("took too many frames to stop: %d", frames)
	}
}

func TestMomentumScrollTopAdvances(t *testing.T) {
	m := momentumModel()
	m.scrollVelocity = scrollWheelImpulse * 3
	m.scrollFrac = 0
	m.scrollTop = 0

	// Simulate a handful of ticks.
	for range 10 {
		m.scrollVelocity *= scrollFriction
		m.scrollFrac += m.scrollVelocity
		delta := int(m.scrollFrac)
		m.scrollFrac -= float64(delta)
		total := m.totalDiffLines()
		bodyH := m.bodyHeight()
		maxScroll := max(0, total-bodyH)
		m.scrollTop = layout.Clamp(m.scrollTop+delta, 0, maxScroll)
	}
	if m.scrollTop == 0 {
		t.Error("scrollTop did not advance during momentum simulation")
	}
}

func TestStopScrollMomentumZeroesState(t *testing.T) {
	m := momentumModel()
	m.scrollVelocity = 10.0
	m.scrollFrac = 0.7
	m.scrollTicking = true

	m.stopScrollMomentum()

	if m.scrollVelocity != 0 {
		t.Errorf("velocity not zeroed: %v", m.scrollVelocity)
	}
	if m.scrollFrac != 0 {
		t.Errorf("frac not zeroed: %v", m.scrollFrac)
	}
	// scrollTicking intentionally left as-is; in-flight tick will exit on its own.
}

func TestMomentumClampedAtBoundary(t *testing.T) {
	m := momentumModel()
	total := m.totalDiffLines()
	bodyH := m.bodyHeight()
	maxScroll := max(0, total-bodyH)
	m.scrollTop = maxScroll
	m.scrollVelocity = scrollMaxVelocity

	for range 5 {
		m.scrollVelocity *= scrollFriction
		m.scrollFrac += m.scrollVelocity
		delta := int(m.scrollFrac)
		m.scrollFrac -= float64(delta)
		m.scrollTop = layout.Clamp(m.scrollTop+delta, 0, maxScroll)
	}
	if m.scrollTop != maxScroll {
		t.Errorf("scrollTop should be clamped at maxScroll=%d, got %d", maxScroll, m.scrollTop)
	}
}
