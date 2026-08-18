package algorithms

// This file holds the patience diff algorithm: Myers run over each range's
// unique-by-value elements to find anchors, with the gaps around and between
// those anchors recursively diffed by Myers too. Time is
// O(N log N + M log M + (N+M)D); space is O(N+M).
//
// It tends to give more human-readable outputs than plain Myers, since an
// anchor is a line that appears exactly once on each side, which is unlikely
// to be a coincidental match. See Bram Cohen's blog post describing it:
// https://bramcohen.livejournal.com/73318.html
//
// It ports the Rust `similar` crate's patience.rs. Three things differ, each
// marked at its site below: there is no Replace-coalescing wrapper (this
// package never produces a Replace, matching Myers and the LCS table diff),
// there is no raw (heuristic-free) Myers variant, and the equal-length /
// full-common-prefix shortcut taken before computing uniqueness is not
// ported — the shared disjoint-range fast path below covers the case that
// shortcut existed for cheaply enough that a second, patience-specific check
// isn't worth the complexity.

import (
	"context"

	"github.com/mahibulhaque/similar/internal/diffutil"
)

// DiffPatience runs the patience diff over the given sub-ranges without a
// deadline.
func DiffPatience[T comparable](
	d diffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
) error {
	return DiffPatienceDeadline(context.Background(), d, old, oldStart, oldEnd, new, newStart, newEnd)
}

// DiffPatienceDeadline runs the patience diff, honoring ctx's deadline and
// cancellation. Every nested Myers diff it runs shares that same context, so
// a bailout anywhere below emits an approximate (delete+insert) script for
// its un-diffed middle, the same as a direct Myers bailout would.
func DiffPatienceDeadline[T comparable](
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

	// Anchors: elements that occur exactly once in each full range. Running
	// Myers over just these (matched by value, via diffutil.Unique) finds a
	// diff shaped around the lines least likely to be coincidental matches.
	oldVals, oldIdx := diffutil.UniqueElements(old, oldStart, oldEnd)
	newVals, newIdx := diffutil.UniqueElements(new, newStart, newEnd)

	p := &patienceHook[T]{
		d:   d,
		ctx: ctx,

		old:        old,
		oldCurrent: oldStart,
		oldEnd:     oldEnd,
		oldIdx:     oldIdx,

		new:        new,
		newCurrent: newStart,
		newEnd:     newEnd,
		newIdx:     newIdx,
	}

	if err := DiffMyersDeadline(ctx, p, oldVals, 0, len(oldVals), newVals, 0, len(newVals)); err != nil {
		return err
	}

	// Whatever remains after the last anchor (or the whole range, if no
	// anchors were found) still needs diffing. Passing d directly here — not
	// wrapped in noFinishHook — is what calls Finish exactly once, since
	// DiffMyersDeadline always calls it on the hook it's given.
	return DiffMyersDeadline(p.ctx, p.d, p.old, p.oldCurrent, p.oldEnd, p.new, p.newCurrent, p.newEnd)
}

// patienceHook receives the edit script Myers produces over the anchor
// arrays (oldVals/newVals, indexed via oldIdx/newIdx), and translates each
// Equal run into real operations on the original ranges.
//
// Delete and Insert, over the anchor arrays, name anchors with no
// counterpart on the other side. Nothing is emitted for them directly — the
// gap they mark gets covered by the nested Myers diff that the surrounding
// Equal calls (or the final diff in DiffPatienceDeadline) run instead. The
// crate leaves delete/insert as its DiffHook trait's default (no-op)
// implementations for the same reason.
type patienceHook[T comparable] struct {
	d   diffHook
	ctx context.Context

	old        []T
	oldCurrent int
	oldEnd     int
	oldIdx     []int // oldIdx[i] is the original index in old of anchor i

	new        []T
	newCurrent int
	newEnd     int
	newIdx     []int // newIdx[i] is the original index in new of anchor i
}

// Equal handles a run of `length` matched anchors starting at oldAnchor and
// newAnchor. Each anchor is processed in turn: first the equal run starting
// at the current cursors is greedily extended by literal comparison (picking
// up incidental matches that aren't themselves unique, including the
// previous anchor, since old[oldCurrent] == new[newCurrent] there by
// construction); then the remaining gap up to the anchor is diffed by a
// nested Myers call; then the cursors advance to the anchor itself, ready
// for the next iteration (or Finish) to consume it.
func (p *patienceHook[T]) Equal(oldAnchor, newAnchor, length int) error {
	for i := range length {
		targetOld := p.oldIdx[oldAnchor+i]
		targetNew := p.newIdx[newAnchor+i]

		a0, b0 := p.oldCurrent, p.newCurrent
		for p.oldCurrent < targetOld && p.newCurrent < targetNew &&
			p.new[p.newCurrent] == p.old[p.oldCurrent] {
			p.oldCurrent++
			p.newCurrent++
		}
		if p.oldCurrent > a0 {
			if err := p.d.Equal(a0, b0, p.oldCurrent-a0); err != nil {
				return err
			}
		}

		nf := noFinishHook{p.d}
		if err := DiffMyersDeadline(p.ctx, nf, p.old, p.oldCurrent, targetOld, p.new, p.newCurrent, targetNew); err != nil {
			return err
		}
		p.oldCurrent, p.newCurrent = targetOld, targetNew
	}
	return nil
}

func (p *patienceHook[T]) Delete(_, _, _ int) error { return nil }
func (p *patienceHook[T]) Insert(_, _, _ int) error { return nil }
func (p *patienceHook[T]) Finish() error            { return nil }

// noFinishHook wraps a diffHook so a nested Myers diff can run to completion
// — Myers always calls Finish on the hook it's given — without that call
// reaching the real hook early. Only the final diff in DiffPatienceDeadline
// is given the real hook directly, so Finish reaches it exactly once.
type noFinishHook struct {
	d diffHook
}

func (n noFinishHook) Equal(oldIndex, newIndex, length int) error {
	return n.d.Equal(oldIndex, newIndex, length)
}

func (n noFinishHook) Delete(oldIndex, oldLen, newIndex int) error {
	return n.d.Delete(oldIndex, oldLen, newIndex)
}

func (n noFinishHook) Insert(oldIndex, newIndex, newLen int) error {
	return n.d.Insert(oldIndex, newIndex, newLen)
}

func (n noFinishHook) Finish() error { return nil }
