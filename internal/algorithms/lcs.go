package algorithms

// This file holds the classic LCS table diff: one table of longest-common-
// subsequence lengths over the trimmed middle of the two ranges, walked forward
// to emit one operation per element. Time and space are both O(N*M), which is
// what the table gate on limits bounds.
//
// It ports the Rust `similar` crate's lcs.rs. Four things differ, each marked
// at its site below: the table is a flat slice rather than a map of non-zero
// cells, it is gated by lcsTableMaxWork, the all-equal shortcut emits at the
// range starts rather than at zero, and the table-declined path emits its
// replacement once.

import (
	"context"

	"github.com/mahibulhaque/similar/internal/diffutil"
)

// DiffLCS runs the LCS table diff over the given sub-ranges without a deadline.
func DiffLCS[T comparable](
	d diffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
) error {
	return DiffLCSDeadline(context.Background(), d, old, oldStart, oldEnd, new, newStart, newEnd)
}

// DiffLCSDeadline runs the LCS table diff, honoring ctx's deadline and
// cancellation. On a bailout it emits an approximate (delete+insert) script for
// the un-diffed middle rather than erroring.
//
// A deadline is a poor fit for this algorithm — the table is built before any
// operation is emitted, so a deadline either costs the whole table or nothing —
// and the crate says as much. The bound worth setting here is on size, and that
// one is not the caller's to pass: see lcsTableMaxWork.
func DiffLCSDeadline[T comparable](
	ctx context.Context,
	d diffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
) error {
	lim := fromContext(ctx)

	emitted, err := maybeEmitDisjointFastPath(d, old, oldStart, oldEnd, new, newStart, newEnd, lim)
	if err != nil {
		return err
	}
	if emitted {
		return nil
	}

	return diffLCS(d, old, oldStart, oldEnd, new, newStart, newEnd, lim)
}

func diffLCS[T comparable](
	d diffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
	lim limits,
) error {
	oldLen := oldEnd - oldStart
	newLen := newEnd - newStart

	oldEmpty := diffutil.IsEmptyRange(oldStart, oldEnd)
	newEmpty := diffutil.IsEmptyRange(newStart, newEnd)
	switch {
	case oldEmpty && newEmpty:
		// The crate emits a zero-length delete here. Emitting nothing keeps a
		// zero-length operation out of every consumer, and matches Myers.
		return d.Finish()
	case newEmpty:
		if err := d.Delete(oldStart, oldLen, newStart); err != nil {
			return err
		}
		return d.Finish()
	case oldEmpty:
		if err := d.Insert(oldStart, newStart, newLen); err != nil {
			return err
		}
		return d.Finish()
	}

	cpl := diffutil.CommonPrefixLen(old, oldStart, oldEnd, new, newStart, newEnd)
	csl := diffutil.CommonSuffixLen(old, oldStart+cpl, oldEnd, new, newStart+cpl, newEnd)

	// The two ranges are identical.
	//
	// The crate emits equal(0, 0, len) here — hard-coded zeros, which report the
	// wrong span for a sub-range diff. Every operation this package emits is in
	// absolute coordinates, so this one is too.
	if cpl == oldLen && oldLen == newLen {
		if err := d.Equal(oldStart, newStart, oldLen); err != nil {
			return err
		}
		return d.Finish()
	}

	// The middle: what is left once the common prefix and suffix are peeled.
	midOldStart, midOldEnd := oldStart+cpl, oldEnd-csl
	midNewStart, midNewEnd := newStart+cpl, newEnd-csl
	midOldLen := midOldEnd - midOldStart
	midNewLen := midNewEnd - midNewStart

	table, stride := makeLCSTable(old, midOldStart, midOldEnd, new, midNewStart, midNewEnd, lim)

	if cpl > 0 {
		if err := d.Equal(oldStart, newStart, cpl); err != nil {
			return err
		}
	}

	// Walk the table forward, one element per operation. A match is always on
	// some longest common subsequence, so it is taken greedily; otherwise the
	// larger of the two neighbours says which side to advance, and a tie deletes.
	//
	// A declined table skips the walk with both cursors at zero, which leaves
	// the whole middle to the tail emits below — one Delete and one Insert, the
	// same approximation Myers makes on a bailout. (The crate emits that pair
	// here and then leaves its cursors unadvanced, so the tails emit it a second
	// time and the script no longer reconstructs.)
	oldIdx, newIdx := 0, 0
	if table != nil {
		for newIdx < midNewLen && oldIdx < midOldLen {
			oldOrigIdx := midOldStart + oldIdx
			newOrigIdx := midNewStart + newIdx

			switch {
			case new[newOrigIdx] == old[oldOrigIdx]:
				if err := d.Equal(oldOrigIdx, newOrigIdx, 1); err != nil {
					return err
				}
				oldIdx++
				newIdx++
			case table[newIdx*stride+oldIdx+1] >= table[(newIdx+1)*stride+oldIdx]:
				if err := d.Delete(oldOrigIdx, 1, newOrigIdx); err != nil {
					return err
				}
				oldIdx++
			default:
				if err := d.Insert(oldOrigIdx, newOrigIdx, 1); err != nil {
					return err
				}
				newIdx++
			}
		}
	}

	if oldIdx < midOldLen {
		if err := d.Delete(midOldStart+oldIdx, midOldLen-oldIdx, midNewStart+newIdx); err != nil {
			return err
		}
		oldIdx = midOldLen
	}

	if newIdx < midNewLen {
		if err := d.Insert(midOldStart+oldIdx, midNewStart+newIdx, midNewLen-newIdx); err != nil {
			return err
		}
	}

	if csl > 0 {
		if err := d.Equal(midOldEnd, midNewEnd, csl); err != nil {
			return err
		}
	}

	return d.Finish()
}

// makeLCSTable builds the table of longest-common-subsequence lengths for every
// pair of suffixes of the two ranges: cell (i, j) is the LCS length of
// new[newStart+i:newEnd] and old[oldStart+j:oldEnd]. It is filled backwards, so
// each cell reads only cells already written, and the returned stride is the row
// width. The table is nil if it was declined, either for exceeding
// lcsTableMaxWork or on a deadline.
//
// The crate stores only non-zero cells, in a map keyed by the pair, and falls
// back on a zero for a missing one. One flat slice with a trailing row and
// column of zeros holds the same values, and holds them in the memory the gate
// can predict from the range lengths alone.
func makeLCSTable[T comparable](
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
	lim limits,
) (table []int32, stride int) {
	oldLen := oldEnd - oldStart
	newLen := newEnd - newStart
	stride = oldLen + 1

	if diffutil.SatMul(stride, newLen+1) > lim.lcsTableMaxWork {
		return nil, 0
	}

	table = make([]int32, stride*(newLen+1))
	for i := newLen - 1; i >= 0; i-- {
		// Are we running for too long? Give up on the table. One check per row:
		// a row is O(oldLen) work, which is what makes reading the clock here
		// cheap enough to do unconditionally.
		if lim.exceeded() {
			return nil, 0
		}

		row := i * stride
		below := row + stride
		for j := oldLen - 1; j >= 0; j-- {
			if new[newStart+i] == old[oldStart+j] {
				table[row+j] = table[below+j+1] + 1
			} else {
				table[row+j] = max(table[below+j], table[row+j+1])
			}
		}
	}

	return table, stride
}
