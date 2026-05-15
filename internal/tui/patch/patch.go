package patch

// patch.go — annotation utilities: matching annotations to unk headers.

import (
	"slices"
	"strings"

	"github.com/kpango/unk/internal/types"
)

// FileSectionLineCount returns the number of terminal lines a file section occupies
// in unified layout (one line per patch element plus a header line).
// Used by navigation and prewarm height estimation.
func FileSectionLineCount(f types.DiffFile) int {
	if f.IsBinary || f.IsTooLarge {
		return 2 // header + "content skipped"
	}
	return strings.Count(f.Patch, "\n") + 2
}

// AnnotationMatchesUnk returns true when the annotation's range overlaps the unk's visible line extent.
func AnnotationMatchesUnk(ann types.AgentAnnotation, h types.DiffUnk) bool {
	if ann.NewRange != nil && h.NewRange != nil {
		hStart := h.NewRange[0]
		count := max(h.NewRange[1], 1)
		hEnd := hStart + count - 1
		annStart, annEnd := ann.NewRange[0], ann.NewRange[1]
		if annStart <= hEnd && hStart <= annEnd {
			return true
		}
	}
	if ann.OldRange != nil && h.OldRange != nil {
		hStart := h.OldRange[0]
		count := max(h.OldRange[1], 1)
		hEnd := hStart + count - 1
		annStart, annEnd := ann.OldRange[0], ann.OldRange[1]
		if annStart <= hEnd && hStart <= annEnd {
			return true
		}
	}
	return false
}

// UnkHasAnnotation reports whether the unk at unkIndex in f has at least one matching annotation.
func UnkHasAnnotation(f types.DiffFile, unkIndex int) bool {
	if unkIndex < 0 || unkIndex >= len(f.Metadata.Unks) || f.Agent == nil {
		return false
	}
	h := f.Metadata.Unks[unkIndex]
	return slices.ContainsFunc(f.Agent.Annotations, func(ann types.AgentAnnotation) bool {
		return AnnotationMatchesUnk(ann, h)
	})
}
