package algorithms

import (
	"github.com/mahibulhaque/similar/internal/diffutil"
)

// maybeEmitDisjointFastPath short-circuits two large ranges that share no
// common item to a straight delete+insert replacement, avoiding full search
// cost. It reports whether it emitted a complete script (in which case Finish
// was already called).
//
// The Rust FNV-1a hash bucketing exists only because that port keys maps by
// u64; here T is comparable, so membership is a direct map lookup.
func maybeEmitDisjointFastPath[T comparable](
	d diffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
	lim limits,
) (bool, error) {
	if lim.exceeded() {
		return false, nil
	}

	oldLen := oldEnd - oldStart
	newLen := newEnd - newStart

	if oldLen < lim.disjointFastPathMinLen ||
		newLen < lim.disjointFastPathMinLen ||
		diffutil.SatMul(oldLen, newLen) < lim.disjointFastPathMinWork {
		return false, nil
	}

	// If the endpoints already match the ranges are not disjoint.
	if new[newStart] == old[oldStart] || new[newEnd-1] == old[oldEnd-1] {
		return false, nil
	}

	common, ok := hasCommonItem(old, oldStart, oldEnd, new, newStart, newEnd, lim)
	if !ok {
		// deadline hit while scanning
		return false, nil
	}
	if common {
		return false, nil
	}

	if err := d.Delete(oldStart, oldLen, newStart); err != nil {
		return false, err
	}
	if err := d.Insert(oldStart, newStart, newLen); err != nil {
		return false, err
	}
	if err := d.Finish(); err != nil {
		return false, err
	}
	return true, nil
}

// hasCommonItem reports whether the two ranges share any element. The second
// result is false if the deadline was hit before the answer was known.
func hasCommonItem[T comparable](
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
	lim limits,
) (common bool, ok bool) {
	seen := make(map[T]struct{}, oldEnd-oldStart)
	for i := oldStart; i < oldEnd; i++ {
		if ((i-oldStart)&(disjointFastPathDeadlineCheckStep-1)) == 0 && lim.exceeded() {
			return false, false
		}
		seen[old[i]] = struct{}{}
	}
	for i := newStart; i < newEnd; i++ {
		if ((i-newStart)&(disjointFastPathDeadlineCheckStep-1)) == 0 && lim.exceeded() {
			return false, false
		}
		if _, found := seen[new[i]]; found {
			return true, true
		}
	}
	return false, true
}
