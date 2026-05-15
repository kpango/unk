// Adapted from github.com/svent/sift (GPL-3.0).
// Original: Copyright (C) 2014-2016 Sven Taute
// Adapts getMatches and countLines from sift/matching.go into exported library
// functions, removing the global-state dependencies that made them package-main only.

package siftlib

import (
	"bytes"
	"regexp"
	"sort"
)

// FindMatches finds all matches of re in data and returns them with per-match
// line start/end positions and extracted line text. Adapted from sift's getMatches
// in matching.go, operating in single-line (non-multiline) mode.
//
// data is the original buffer; line text is extracted from it.
// testData is the buffer the regex is applied to — pass nil to use data directly,
// or pass a BytesToLower-transformed copy for ASCII case-insensitive matching
// without the overhead of Go's (?i) flag (sift's --ignore-case approach).
//
// After match positions are found, AssignLineNumbers is called automatically
// to populate Match.LineNo using sift's countLines walk.
func FindMatches(data []byte, testData []byte, re *regexp.Regexp) Matches {
	if re == nil || len(data) == 0 {
		return nil
	}
	if testData == nil {
		testData = data
	}
	length := len(data)

	// Bulk FindAllIndex over the full buffer — sift's getMatches approach.
	allIndex := re.FindAllIndex(testData, -1)
	if allIndex == nil {
		return nil
	}

	var matches Matches
	for mi := 0; mi < len(allIndex); mi++ {
		index := allIndex[mi]
		start := index[0]
		end := index[1]

		// Non-multiline mode: strip leading/trailing newlines from match boundaries,
		// then verify the trimmed range still satisfies the regex. Mirrors sift's
		// getMatches logic that avoids false positives from \s matching '\n'.
		for start < length && end > start && data[start] == 0x0a {
			start++
		}
		for end > 0 && end > start && data[end-1] == 0x0a {
			end--
		}
		if start >= end {
			continue
		}
		if !re.Match(testData[start:end]) {
			continue
		}

		// If the corrected match still spans a newline, split it into per-line
		// sub-matches and re-enqueue them. Adapted from sift's getMatches multiline
		// cross-line handling in single-line mode.
		if bytes.Contains(data[start:end], []byte{0x0a}) {
			lineStart := start
			lineEnd := end
			for lineStart > 0 && data[lineStart-1] != 0x0a {
				lineStart--
			}
			for lineEnd < length && data[lineEnd] != 0x0a {
				lineEnd++
			}
			lastStart := lineStart
			for pos := lastStart + 1; pos < lineEnd; pos++ {
				if data[pos] == 0x0a || pos == lineEnd-1 {
					if pos == lineEnd-1 && data[pos] != 0x0a {
						pos++
					}
					if idx := re.FindIndex(testData[lastStart:pos]); idx != nil {
						allIndex = append(allIndex, []int{lastStart + idx[0], lastStart + idx[1]})
					}
					lastStart = pos + 1
				}
			}
			continue
		}

		// Find the full line containing this match by scanning backward/forward
		// for '\n' boundaries — sift's getMatches line extraction logic.
		lineStart := start
		lineEnd := end
		for lineStart > 0 && data[lineStart-1] != 0x0a {
			lineStart--
		}
		for lineEnd < length && data[lineEnd] != 0x0a {
			lineEnd++
		}

		matches = append(matches, Match{
			Start:     start,
			End:       end,
			LineStart: lineStart,
			LineEnd:   lineEnd,
			Text:      string(data[start:end]),
			Line:      string(data[lineStart:lineEnd]),
		})
	}

	if len(matches) > 1 {
		sort.Sort(Matches(matches))
	}

	// Assign line numbers via sift's countLines algorithm.
	AssignLineNumbers(data, matches)

	return matches
}

// AssignLineNumbers walks data counting '\n' bytes and sets Match.LineNo on
// each entry. Adapted from sift's countLines in matching.go; conditions and
// streaming output logic are removed since they are not needed for library use.
// matches must be sorted by LineStart before calling.
func AssignLineNumbers(data []byte, matches Matches) {
	if len(matches) == 0 {
		return
	}
	lineNo := 0
	mi := 0
	for i := 0; i < len(data) && mi < len(matches); i++ {
		if data[i] == 0x0a {
			// Assign current lineNo to all matches whose line starts at or before i
			// (sift's countLines assigns lineno when the terminating '\n' is found).
			for mi < len(matches) && i >= matches[mi].LineStart {
				matches[mi].LineNo = lineNo
				mi++
			}
			lineNo++
		}
	}
	// Matches on the last line (no trailing '\n') — sift's end-of-buffer check.
	for mi < len(matches) {
		matches[mi].LineNo = lineNo
		mi++
	}
}

// LineNumbers returns the unique set of 0-based line numbers from a sorted
// Matches slice, deduplicating consecutive matches on the same line.
func LineNumbers(matches Matches) []int {
	if len(matches) == 0 {
		return nil
	}
	result := make([]int, 0, len(matches))
	for _, m := range matches {
		if len(result) == 0 || result[len(result)-1] != m.LineNo {
			result = append(result, m.LineNo)
		}
	}
	return result
}
