// Package diffutil holds small helpers shared by the diff algorithms:
// common-prefix/suffix scanning over sub-ranges, an empty-range check, and a
// saturating multiply used to guard the heuristic work gates against overflow.
//
// It is a leaf package: it depends only on the standard library.
package diffutil

import "math"

// IsEmptyRange reports whether the half-open range [start, end) is empty.
//
// A start greater than end is also treated as empty, mirroring the Rust
// crate's is_empty_range.
func IsEmptyRange(start, end int) bool {
	return start >= end
}

// CommonPrefixLen returns the length of the common prefix of old[oldStart:oldEnd]
// and new[newStart:newEnd].
func CommonPrefixLen[T comparable](old []T, oldStart, oldEnd int, new []T, newStart, newEnd int) int {
	if IsEmptyRange(oldStart, oldEnd) || IsEmptyRange(newStart, newEnd) {
		return 0
	}
	maxLen := min(oldEnd-oldStart, newEnd-newStart)
	matched := 0
	for matched < maxLen && new[newStart+matched] == old[oldStart+matched] {
		matched++
	}
	return matched
}

// CommonSuffixLen returns the length of the common suffix of old[oldStart:oldEnd]
// and new[newStart:newEnd].
func CommonSuffixLen[T comparable](old []T, oldStart, oldEnd int, new []T, newStart, newEnd int) int {
	if IsEmptyRange(oldStart, oldEnd) || IsEmptyRange(newStart, newEnd) {
		return 0
	}
	maxLen := min(oldEnd-oldStart, newEnd-newStart)
	matched := 0
	for matched < maxLen && new[newEnd-1-matched] == old[oldEnd-1-matched] {
		matched++
	}
	return matched
}

// SatMul multiplies two non-negative ints, saturating at math.MaxInt on
// overflow. It guards the work = oldLen * newLen heuristic gates.
func SatMul(a, b int) int {
	if a == 0 || b == 0 {
		return 0
	}
	r := a * b
	if r/a != b || r < 0 {
		return math.MaxInt
	}
	return r
}
