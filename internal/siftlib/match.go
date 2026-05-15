// Adapted from github.com/svent/sift (GPL-3.0).
// Original: Copyright (C) 2014-2016 Sven Taute
// sift's Match/Matches types used unexported fields; exported here for library use.

package siftlib

import "sort"

// Match represents a single pattern match in a searched buffer. Adapted from
// sift's internal Match type (matching.go), with unexported fields promoted to
// exported ones so the struct can be used as a library value.
type Match struct {
	// Start is the byte offset of the first byte of the match within the buffer.
	Start int
	// End is the byte offset one past the last byte of the match.
	End int
	// LineStart is the byte offset of the first byte of the line containing the match.
	LineStart int
	// LineEnd is the byte offset of the last byte of the line (exclusive of '\n').
	LineEnd int
	// Text is the matched substring (extracted from original data, not testData).
	Text string
	// Line is the full text of the line containing the match.
	Line string
	// LineNo is the 0-based line number of the matching line.
	LineNo int
}

// Matches is a slice of Match values. Implements sort.Interface ordered by
// Start offset, mirroring sift's Matches type from matching.go.
type Matches []Match

func (m Matches) Len() int           { return len(m) }
func (m Matches) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m Matches) Less(i, j int) bool { return m[i].Start < m[j].Start }

// Sort sorts matches in ascending byte-offset order.
func (m Matches) Sort() { sort.Sort(m) }
