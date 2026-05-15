// Package siftlib adapts core matching functions from github.com/svent/sift
// (GPL-3.0, Copyright (C) 2014-2016 Sven Taute) into an importable library.
// sift is a package main CLI; this package repackages its buffer-oriented
// getMatches / countLines matching approach and SIMD-accelerated countNewlines /
// bytesToLower helpers so they can be used as a library dependency.
package siftlib

import (
	"regexp"
	"strings"
)

// CompilePattern compiles pattern as a case-insensitive regular expression,
// mirroring sift's --ignore-case behavior ((?i) prefix). If the pattern is not
// valid regex syntax it is quoted so it matches literally, matching sift's
// treatment of non-regex patterns. Returns nil for empty patterns.
func CompilePattern(pattern string) *regexp.Regexp {
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		// Invalid regex: fall back to literal string matching.
		re = regexp.MustCompile("(?i)" + regexp.QuoteMeta(pattern))
	}
	return re
}

// SearchLines returns the 0-based line indices of lines in content that contain
// at least one match of re. It uses FindMatches (sift's getMatches approach) over
// the full buffer, then AssignLineNumbers (sift's countLines) to compute line
// indices, and finally deduplicates via LineNumbers.
//
// content should be plain text with ANSI escape sequences stripped by the caller.
func SearchLines(content []byte, re *regexp.Regexp) []int {
	return LineNumbers(FindMatches(content, nil, re))
}

// SearchLinesCI performs case-insensitive search using sift's BytesToLower approach:
// the buffer is ASCII-lowercased in a scratch copy via BytesToLower (the SIMD
// path on amd64), and the pattern is matched against the lowercased copy while
// line text is extracted from the original. This is sift's actual --ignore-case
// implementation — faster than the (?i) regex flag for large ASCII buffers since
// the regex engine operates on pre-lowercased bytes without per-character case folding.
//
// Unlike CompilePattern + SearchLines, this function does NOT support Unicode
// case folding — it is intentionally ASCII-only, matching sift's behaviour.
func SearchLinesCI(content []byte, pattern string) []int {
	if len(content) == 0 || pattern == "" {
		return nil
	}

	// Pre-lowercase the buffer using sift's bytesToLower (SIMD on amd64).
	testBuf := make([]byte, len(content))
	BytesToLower(content, testBuf)

	// Compile the pattern against the already-lowercased buffer — no (?i) needed.
	lp := strings.ToLower(pattern)
	re, err := regexp.Compile(lp)
	if err != nil {
		re = regexp.MustCompile(regexp.QuoteMeta(lp))
	}

	return LineNumbers(FindMatches(content, testBuf, re))
}

// CountNewlines returns the number of '\n' bytes in input.
// On amd64 this calls the SIMD-accelerated implementation from sift's
// matching_amd64.s; other platforms use the pure-Go fallback.
func CountNewlines(input []byte) int {
	return countNewlines(input, len(input))
}

// BytesToLower ASCII-lowercases input into output (must be same length).
// On amd64 this calls the SIMD-accelerated implementation from sift's
// matching_amd64.s; other platforms use the pure-Go lookup-table fallback.
func BytesToLower(input, output []byte) {
	bytesToLower(input, output, len(input))
}
