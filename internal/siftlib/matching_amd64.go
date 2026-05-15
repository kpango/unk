// Adapted from github.com/svent/sift (GPL-3.0).
// Original: Copyright (C) 2014-2016 Sven Taute
// Package changed from main → siftlib to allow library use.

//go:build amd64

package siftlib

// countNewlines counts '\n' bytes in input[:length] using the SIMD-accelerated
// implementation in matching_amd64.s (copied from sift).
func countNewlines(input []byte, length int) int

// bytesToLower converts ASCII uppercase bytes to lowercase in-place using the
// SIMD-accelerated implementation in matching_amd64.s (copied from sift).
func bytesToLower(input []byte, output []byte, length int)
