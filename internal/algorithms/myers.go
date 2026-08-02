// Package algorithms holds the Myers' diff implementation and its heuristics.
//
// It ports the Rust `similar` crate's Myers algorithm behavior-for-behavior:
// classic divide-and-conquer middle-snake recursion, front-anchor peeling,
// exact small-side fallback (both directions), the disjoint fast path, and a
// deadline bailout. It is internal; the public facade lives in package similar.
package algorithms

import (
	"context"

	"github.com/mahibulhaque/similar/internal/diffutil"
)

const (
	smallSideExactMax              = 64
	smallSideExactMinLarge         = 512
	smallSideExactMaxWork          = 64_000_000
	smallSideDeadlineCheckInterval = 1024
	frontAnchorDeadlineCheckStep   = 1024
)

// reachVector records the endpoints of the furthest-reaching D-paths, indexed
// by diagonal k (which can be negative). It replaces Rust's operator-overloaded
// Index<isize>: offset maps negative k into a non-negative slice index.
type reachVector struct {
	offset int
	data   []int
}

func newReachVector(maxD int) *reachVector {
	return &reachVector{offset: maxD, data: make([]int, 2*maxD)}
}

func (v *reachVector) at(k int) int { return v.data[k+v.offset] }
func (v *reachVector) set(k, x int) { v.data[k+v.offset] = x }
func (v *reachVector) length() int  { return len(v.data) }

func maxD(len1, len2 int) int {
	// (len1 + len2) rounded up to the nearest even half, plus one.
	return (len1+len2+1)/2 + 1
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Diff runs Myers' diff over the given sub-ranges without a deadline.
func Diff[T comparable](
	d diffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
) error {
	return DiffDeadline(context.Background(), d, old, oldStart, oldEnd, new, newStart, newEnd)
}

// DiffDeadline runs Myers' diff, honoring ctx's deadline and cancellation. On a
// bailout it emits an approximate (delete+insert) script for the un-diffed
// middle rather than erroring.
func DiffDeadline[T comparable](
	ctx context.Context,
	d diffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
) error {
	dl := fromContext(ctx)

	emitted, err := maybeEmitDisjointFastPath(d, old, oldStart, oldEnd, new, newStart, newEnd, dl)
	if err != nil {
		return err
	}
	if emitted {
		return nil
	}

	maxD := maxD(oldEnd-oldStart, newEnd-newStart)
	vf := newReachVector(maxD)
	vb := newReachVector(maxD)
	if err := conquer(d, old, oldStart, oldEnd, new, newStart, newEnd, vf, vb, dl); err != nil {
		return err
	}
	return d.Finish()
}

func conquer[T comparable](
	d diffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
	vf, vb *reachVector,
	dl deadline,
) error {
	// Peel the common prefix.
	cpl := diffutil.CommonPrefixLen(old, oldStart, oldEnd, new, newStart, newEnd)
	if cpl > 0 {
		if err := d.Equal(oldStart, newStart, cpl); err != nil {
			return err
		}
	}
	oldStart += cpl
	newStart += cpl

	// Peel the common suffix (emitted last, after the middle).
	csl := diffutil.CommonSuffixLen(old, oldStart, oldEnd, new, newStart, newEnd)
	csOld := oldEnd - csl
	csNew := newEnd - csl
	oldEnd -= csl
	newEnd -= csl

	// Front-anchor peel for heavily unbalanced shifts.
	for oldStart < oldEnd && newStart < newEnd {
		oldBefore, newBefore := oldStart, newStart
		var err error
		oldStart, newStart, err = tryEmitFrontAnchor(d, old, oldStart, oldEnd, new, newStart, newEnd, dl)
		if err != nil {
			return err
		}
		if oldStart == oldBefore && newStart == newBefore {
			break
		}
	}

	switch {
	case diffutil.IsEmptyRange(oldStart, oldEnd) && diffutil.IsEmptyRange(newStart, newEnd):
		// nothing to do
	case diffutil.IsEmptyRange(newStart, newEnd):
		if err := d.Delete(oldStart, oldEnd-oldStart, newStart); err != nil {
			return err
		}
	case diffutil.IsEmptyRange(oldStart, oldEnd):
		if err := d.Insert(oldStart, newStart, newEnd-newStart); err != nil {
			return err
		}
	default:
		emitted, err := maybeEmitSmallSideExact(d, old, oldStart, oldEnd, new, newStart, newEnd, dl)
		if err != nil {
			return err
		}
		if emitted {
			break
		}
		if xStart, yStart, ok := findMiddleSnake(old, oldStart, oldEnd, new, newStart, newEnd, vf, vb, dl); ok {
			if err := conquer(d, old, oldStart, xStart, new, newStart, yStart, vf, vb, dl); err != nil {
				return err
			}
			if err := conquer(d, old, xStart, oldEnd, new, yStart, newEnd, vf, vb, dl); err != nil {
				return err
			}
		} else {
			// Deadline reached: approximate the un-diffed middle.
			if err := d.Delete(oldStart, oldEnd-oldStart, newStart); err != nil {
				return err
			}
			if err := d.Insert(oldStart, newStart, newEnd-newStart); err != nil {
				return err
			}
		}
	}

	if csl > 0 {
		if err := d.Equal(csOld, csNew, csl); err != nil {
			return err
		}
	}
	return nil
}

// findMiddleSnake simultaneously runs the forward and reverse searches until
// the furthest-reaching paths overlap, returning the snake's start point in
// absolute (old, new) coordinates. The third result is false if the deadline
// was reached first.
func findMiddleSnake[T comparable](
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
	vf, vb *reachVector,
	dl deadline,
) (int, int, bool) {
	n := oldEnd - oldStart
	m := newEnd - newStart

	delta := n - m
	odd := delta&1 == 1

	vf.set(1, 0)
	vb.set(1, 0)

	dMax := maxD(n, m)
	_ = vf.length() // capacity invariant: length() >= dMax by construction

	for d := 0; d < dMax; d++ {
		if dl.exceeded() {
			break
		}

		// Forward path.
		for k := d; k >= -d; k -= 2 {
			var x int
			if k == -d || (k != d && vf.at(k-1) < vf.at(k+1)) {
				x = vf.at(k + 1)
			} else {
				x = vf.at(k-1) + 1
			}
			y := x - k
			x0, y0 := x, y
			if x < n && y < m {
				x += diffutil.CommonPrefixLen(old, oldStart+x, oldEnd, new, newStart+y, newEnd)
			}
			vf.set(k, x)
			if odd && absInt(k-delta) <= d-1 {
				if vf.at(k)+vb.at(-(k-delta)) >= n {
					return x0 + oldStart, y0 + newStart, true
				}
			}
		}

		// Backward path.
		for k := d; k >= -d; k -= 2 {
			var x int
			if k == -d || (k != d && vb.at(k-1) < vb.at(k+1)) {
				x = vb.at(k + 1)
			} else {
				x = vb.at(k-1) + 1
			}
			y := x - k
			if x < n && y < m {
				advance := diffutil.CommonSuffixLen(old, oldStart, oldStart+n-x, new, newStart, newStart+m-y)
				x += advance
				y += advance
			}
			vb.set(k, x)
			if !odd && absInt(k-delta) <= d {
				if vb.at(k)+vf.at(-(k-delta)) >= n {
					return n - x + oldStart, m - y + newStart, true
				}
			}
		}
	}

	return 0, 0, false
}

func tryEmitFrontAnchor[T comparable](
	d diffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
	dl deadline,
) (int, int, error) {
	const maxSkip = 4
	const minAnchorCommon = 96

	oldLen := oldEnd - oldStart
	newLen := newEnd - newStart

	if oldLen <= minAnchorCommon || newLen <= minAnchorCommon {
		return oldStart, newStart, nil
	}

	// Only worthwhile when one side is substantially larger than the other.
	small := min(oldLen, newLen)
	large := max(oldLen, newLen)
	if large < diffutil.SatMul(small, 2) {
		return oldStart, newStart, nil
	}

	maxOldSkip := min(oldLen-1, maxSkip)
	maxNewSkip := min(newLen-1, maxSkip)

	preferInsertAnchor := newLen > oldLen
	preferDeleteAnchor := oldLen > newLen

	var best struct {
		oldSkip, newSkip, common int
		ok                       bool
	}

search:
	for oldSkip := 0; oldSkip <= maxOldSkip; oldSkip++ {
		for newSkip := 0; newSkip <= maxNewSkip; newSkip++ {
			if dl.exceeded() {
				return oldStart, newStart, nil
			}
			if oldSkip == 0 && newSkip == 0 {
				continue
			}
			// Only peel one-sided shifts, from the larger side, to keep the
			// heuristic exactness-preserving.
			if oldSkip != 0 && newSkip != 0 {
				continue
			}
			if preferInsertAnchor && oldSkip != 0 {
				continue
			}
			if preferDeleteAnchor && newSkip != 0 {
				continue
			}
			if new[newStart+newSkip] != old[oldStart+oldSkip] {
				continue
			}

			common, ok := commonPrefixLenAtDeadline(old, oldStart+oldSkip, oldEnd, new, newStart+newSkip, newEnd, dl)
			if !ok {
				return oldStart, newStart, nil
			}
			if common >= minAnchorCommon {
				if common >= minAnchorCommon*8 {
					best.oldSkip, best.newSkip, best.common, best.ok = oldSkip, newSkip, common, true
					break search
				}
				if !best.ok || betterAnchor(common, oldSkip, newSkip, best.common, best.oldSkip, best.newSkip) {
					best.oldSkip, best.newSkip, best.common, best.ok = oldSkip, newSkip, common, true
				}
			}
		}
	}

	if !best.ok {
		return oldStart, newStart, nil
	}

	// The search skips the (0,0) candidate and every two-sided one, so a
	// recorded best always has exactly one skip zero and peels a single side.
	if best.oldSkip == 0 {
		if err := d.Insert(oldStart, newStart, best.newSkip); err != nil {
			return oldStart, newStart, err
		}
	} else {
		if err := d.Delete(oldStart, best.oldSkip, newStart); err != nil {
			return oldStart, newStart, err
		}
	}

	if err := d.Equal(oldStart+best.oldSkip, newStart+best.newSkip, best.common); err != nil {
		return oldStart, newStart, err
	}
	return oldStart + best.oldSkip + best.common, newStart + best.newSkip + best.common, nil
}

// betterAnchor implements the crate's candidate ordering: prefer a longer
// common run, then a shorter skip, then larger old/new skips. Candidates are
// one-sided, so the skip compared is whichever of the two is non-zero.
func betterAnchor(common, oldSkip, newSkip, bestCommon, bestOldSkip, bestNewSkip int) bool {
	if common != bestCommon {
		return common > bestCommon
	}
	rs := max(oldSkip, newSkip)
	brs := max(bestOldSkip, bestNewSkip)
	if rs != brs {
		return rs < brs
	}
	if oldSkip != bestOldSkip {
		return oldSkip > bestOldSkip
	}
	return newSkip > bestNewSkip
}

func commonPrefixLenAtDeadline[T comparable](
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
	dl deadline,
) (int, bool) {
	maxLen := min(oldEnd-oldStart, newEnd-newStart)
	matched := 0
	for matched < maxLen {
		if matched&(frontAnchorDeadlineCheckStep-1) == 0 && dl.exceeded() {
			return 0, false
		}
		if new[newStart+matched] != old[oldStart+matched] {
			break
		}
		matched++
	}
	return matched, true
}
