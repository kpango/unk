package model

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kpango/unk/internal/tui/patch"
	"github.com/kpango/unk/internal/tui/render"
	"github.com/kpango/unk/internal/tui/textutil"
	"github.com/kpango/unk/internal/types"
)

func TestParseUnkHeader(t *testing.T) {
	tests := []struct {
		line    string
		wantOld int
		wantNew int
	}{
		{"@@ -1,5 +1,6 @@", 1, 1},
		{"@@ -10,3 +12,7 @@ func foo() {", 10, 12},
		{"@@ -1 +1 @@", 1, 1},
		{"@@ -0,0 +1,3 @@", 0, 1},
		{"not a unk header", -1, -1},
		{"@@ @@", -1, -1},
	}
	for _, tt := range tests {
		gotOld, gotNew := patch.ParseUnkHeader(tt.line)
		if gotOld != tt.wantOld || gotNew != tt.wantNew {
			t.Errorf("parseUnkHeader(%q) = (%d,%d), want (%d,%d)",
				tt.line, gotOld, gotNew, tt.wantOld, tt.wantNew)
		}
	}
}

func TestApplyHorizontalOffset(t *testing.T) {
	tests := []struct {
		s      string
		offset int
		want   string
	}{
		{"hello", 0, "hello"},
		{"hello", 2, "llo"},
		{"hello", 5, ""},
		{"hello", 10, ""},
		{"", 3, ""},
		{"héllo", 1, "éllo"},
		{"héllo", 2, "llo"},
	}
	for _, tt := range tests {
		got := textutil.ApplyHorizontalOffset(tt.s, tt.offset)
		if got != tt.want {
			t.Errorf("applyHorizontalOffset(%q, %d) = %q, want %q", tt.s, tt.offset, got, tt.want)
		}
	}
}

func TestChangesetDiffers(t *testing.T) {
	makeCS := func(paths ...string) types.Changeset {
		cs := types.Changeset{}
		for _, p := range paths {
			cs.Files = append(cs.Files, types.DiffFile{Path: p, Stats: types.DiffStats{Additions: 1}})
		}
		return cs
	}

	a := makeCS("foo.go", "bar.go")
	b := makeCS("foo.go", "bar.go")
	c := makeCS("foo.go")
	d := makeCS("foo.go", "bar.go")
	// Modify stats on d.
	d.Files[0].Stats.Additions = 99

	if changesetDiffers(a, b) {
		t.Error("identical changesets should not differ")
	}
	if !changesetDiffers(a, c) {
		t.Error("different file counts should differ")
	}
	if !changesetDiffers(a, d) {
		t.Error("different stats should differ")
	}
}

func TestAnnotationMatchesUnk(t *testing.T) {
	// Unk ranges use [start, count] format (from the diff parser).
	unkRange := func(start, count int) *[2]int { v := [2]int{start, count}; return &v }
	// Annotation ranges use [start, end] inclusive format (TS sidecar / live comment format).
	annRange := func(start, end int) *[2]int { v := [2]int{start, end}; return &v }

	unk := types.DiffUnk{
		OldRange: unkRange(5, 3),  // old lines 5..7
		NewRange: unkRange(10, 5), // new lines 10..14
	}

	tests := []struct {
		name string
		ann  types.AgentAnnotation
		want bool
	}{
		// newRange matching (annotation ranges are [start, end] inclusive)
		{"exact start", types.AgentAnnotation{NewRange: annRange(10, 10)}, true},
		{"mid range", types.AgentAnnotation{NewRange: annRange(12, 12)}, true},
		{"last line", types.AgentAnnotation{NewRange: annRange(14, 14)}, true},
		{"past end", types.AgentAnnotation{NewRange: annRange(15, 15)}, false},
		{"before start", types.AgentAnnotation{NewRange: annRange(9, 9)}, false},
		{"spans unk", types.AgentAnnotation{NewRange: annRange(8, 16)}, true},
		// oldRange matching
		{"old mid range", types.AgentAnnotation{OldRange: annRange(6, 6)}, true},
		{"old past end", types.AgentAnnotation{OldRange: annRange(8, 8)}, false},
		// nil range → no match
		{"nil range", types.AgentAnnotation{}, false},
	}
	for _, tt := range tests {
		got := patch.AnnotationMatchesUnk(tt.ann, unk)
		if got != tt.want {
			t.Errorf("%s: annotationMatchesUnk = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestUnkHasAnnotation(t *testing.T) {
	// Unk ranges: [start, count]. Annotation ranges: [start, end] inclusive.
	unkRange := func(start, count int) *[2]int { v := [2]int{start, count}; return &v }
	annRange := func(start, end int) *[2]int { v := [2]int{start, end}; return &v }

	f := types.DiffFile{
		Metadata: types.DiffMetadata{
			Unks: []types.DiffUnk{
				{NewRange: unkRange(1, 5)},   // lines 1..5
				{NewRange: unkRange(20, 10)}, // lines 20..29
			},
		},
		Agent: &types.AgentFileContext{
			Annotations: []types.AgentAnnotation{
				{NewRange: annRange(22, 22)}, // matches unk 1 (lines 20..29)
			},
		},
	}

	if patch.UnkHasAnnotation(f, 0) {
		t.Error("unk 0 should not have annotation")
	}
	if !patch.UnkHasAnnotation(f, 1) {
		t.Error("unk 1 should have annotation")
	}
	if patch.UnkHasAnnotation(f, 2) {
		t.Error("out-of-range unk index should return false")
	}

	// File with no agent notes.
	fNoAgent := types.DiffFile{
		Metadata: types.DiffMetadata{Unks: []types.DiffUnk{{NewRange: unkRange(1, 5)}}},
	}
	if patch.UnkHasAnnotation(fNoAgent, 0) {
		t.Error("file with no agent should return false")
	}
}

// TestPrewarmSections verifies that cmdPrewarmSections renders all file
// sections via incremental tea.Batch delivery (one render.PrewarmMsg per file),
// and that each render.PrewarmMsg handler correctly merges the section into sectionCache.
func TestPrewarmSections(t *testing.T) {
	m := benchModel(t, 5, 50)
	m.clearRenderCache()
	gen := m.prewarmGen

	// Run the batch command synchronously — it returns tea.BatchMsg.
	cmd := cmdPrewarmSections(m.renderClone(), m.bootstrap.Changeset.Files, gen)
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg (incremental batch), got %T", msg)
	}
	if len(batch) != 5 {
		t.Fatalf("expected 5 per-file cmds, got %d", len(batch))
	}

	// Simulate BubbleTea running each per-file Cmd and delivering its render.PrewarmMsg.
	for i, c := range batch {
		result := c()
		pw, ok := result.(render.PrewarmMsg)
		if !ok {
			t.Fatalf("batch[%d]: expected render.PrewarmMsg, got %T", i, result)
		}
		if pw.Gen != gen {
			t.Fatalf("batch[%d]: generation mismatch: got %d, want %d", i, pw.Gen, gen)
		}
		if len(pw.Sections) != 1 {
			t.Fatalf("batch[%d]: expected 1 section per msg, got %d", i, len(pw.Sections))
		}
		_, _ = m.Update(pw)
	}

	// All sections should now be in the cache.
	for _, f := range m.bootstrap.Changeset.Files {
		key := m.sectionCacheKey(f)
		if _, ok := m.sectionCache[key]; !ok {
			t.Errorf("section not in cache after prewarm: %s", key)
		}
	}

	// Verify stale generation is discarded.
	m.clearRenderCache() // advances prewarmGen
	staleCmd := cmdPrewarmSections(m.renderClone(), m.bootstrap.Changeset.Files, gen)
	if staleCmd != nil {
		// Old gen — run all per-file cmds and simulate delivery with stale gen.
		staleBatch, ok := staleCmd().(tea.BatchMsg)
		if !ok {
			t.Skip("stale prewarm: unexpected msg type")
		}
		for _, c := range staleBatch {
			if pw, ok := c().(render.PrewarmMsg); ok {
				pw.Gen = gen // force stale gen
				_, _ = m.Update(pw)
			}
		}
	}
	// After stale delivery, sectionCache should still be nil (clearRenderCache wiped it,
	// and the stale gen msgs are discarded by the handler).
	for _, f := range m.bootstrap.Changeset.Files {
		key := m.sectionCacheKey(f)
		if _, ok := m.sectionCache[key]; ok {
			t.Errorf("stale prewarm section should not be in cache: %s", key)
		}
	}
}
