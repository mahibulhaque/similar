package similar

import (
	"context"
	"errors"
	"math/rand"
	"reflect"
	"slices"
	"testing"
	"time"
)

func TestNoopHookReturnsNil(t *testing.T) {
	var h NoopHook
	if err := h.Equal(0, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err := h.Delete(0, 1, 0); err != nil {
		t.Fatal(err)
	}
	if err := h.Insert(0, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err := h.Replace(0, 1, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err := h.Finish(); err != nil {
		t.Fatal(err)
	}
}

// overrideOnly embeds NoopHook and overrides a single callback, proving the
// selective-override pattern compiles and works.
type overrideOnly struct {
	NoopHook
	equals int
}

func (o *overrideOnly) Equal(int, int, int) error { o.equals++; return nil }

func TestNoopHookSelectiveOverride(t *testing.T) {
	o := &overrideOnly{}
	_ = o.Equal(0, 0, 1)
	_ = o.Delete(0, 1, 0) // inherited no-op
	if o.equals != 1 {
		t.Fatalf("equals = %d, want 1", o.equals)
	}
}

func TestCaptureAccumulates(t *testing.T) {
	c := NewCapture()
	_ = c.Equal(0, 0, 2)
	_ = c.Delete(2, 1, 2)
	_ = c.Insert(3, 2, 3)
	ops := c.Ops()
	if len(ops) != 3 {
		t.Fatalf("len = %d, want 3", len(ops))
	}
	if ops[0].Tag != Equal || ops[1].Tag != Delete || ops[2].Tag != Insert {
		t.Fatalf("unexpected tags: %v", ops)
	}
}

func TestReplaceHookCoalesces(t *testing.T) {
	c := NewCapture()
	r := NewReplaceHook(c)
	// equal, then delete+insert adjacent (should coalesce into replace).
	_ = r.Equal(0, 0, 3)
	_ = r.Delete(3, 2, 3)
	_ = r.Insert(5, 3, 3)
	_ = r.Finish()

	ops := c.Ops()
	want := []DiffOp{
		{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 3, NewLen: 3},
		{Tag: Replace, OldIndex: 3, NewIndex: 3, OldLen: 2, NewLen: 3},
	}
	if len(ops) != len(want) {
		t.Fatalf("ops = %v, want %v", ops, want)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Fatalf("op %d = %v, want %v", i, ops[i], want[i])
		}
	}
}

func TestReplaceHookLoneDeleteStaysDelete(t *testing.T) {
	c := NewCapture()
	r := NewReplaceHook(c)
	_ = r.Delete(0, 2, 0)
	_ = r.Equal(2, 0, 1)
	_ = r.Finish()
	ops := c.Ops()
	if len(ops) != 2 || ops[0].Tag != Delete || ops[1].Tag != Equal {
		t.Fatalf("ops = %v, want [Delete Equal]", ops)
	}
}

func TestCompactMergesAndReplays(t *testing.T) {
	// compact + Replace should still reconstruct new and stay minimal-cost.
	old := []int{1, 2, 3, 4, 5, 6}
	new := []int{1, 2, 9, 4, 5, 6}
	c := NewCapture()
	comp := newCompact[int](NewReplaceHook(c), old, new)
	// Feed a raw equal/delete/insert script (as the core would).
	_ = comp.Equal(0, 0, 2)
	_ = comp.Delete(2, 1, 2)
	_ = comp.Insert(3, 2, 1)
	_ = comp.Equal(3, 3, 3)
	_ = comp.Finish()

	out := make([]int, 0, len(new))
	for _, op := range c.Ops() {
		switch op.Tag {
		case Equal:
			out = append(out, old[op.OldIndex:op.OldIndex+op.OldLen]...)
		case Insert, Replace:
			out = append(out, new[op.NewIndex:op.NewIndex+op.NewLen]...)
		}
	}
	if len(out) != len(new) {
		t.Fatalf("reconstruct len = %d, want %d (ops=%v)", len(out), len(new), c.Ops())
	}
	for i := range new {
		if out[i] != new[i] {
			t.Fatalf("reconstruct = %v, want %v", out, new)
		}
	}
}

const unknownAlg = Algorithm(99)

func TestCaptureOpsRunsStandardHookStack(t *testing.T) {
	old := []string{"a", "b", "c", "d"}
	new := []string{"a", "x", "y", "d"}

	ops := captureOps(context.Background(), Myers, old, new)

	// Replace is only produced by the Replace hook, so seeing one here proves
	// the delete+insert pair was coalesced rather than passed straight through.
	want := []DiffOp{
		{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 1, NewLen: 1},
		{Tag: Replace, OldIndex: 1, NewIndex: 1, OldLen: 2, NewLen: 2},
		{Tag: Equal, OldIndex: 3, NewIndex: 3, OldLen: 1, NewLen: 1},
	}
	if !reflect.DeepEqual(ops, want) {
		t.Fatalf("ops = %v, want %v", ops, want)
	}
}

func TestCaptureOpsIdenticalSequences(t *testing.T) {
	s := []int{1, 2, 3}
	ops := captureOps(context.Background(), Myers, s, s)
	want := []DiffOp{{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 3, NewLen: 3}}
	if !reflect.DeepEqual(ops, want) {
		t.Fatalf("ops = %v, want %v", ops, want)
	}
}

func TestCaptureOpsEmptyInputs(t *testing.T) {
	ops := captureOps(context.Background(), Myers, []int{}, []int{})
	if len(ops) != 0 {
		t.Fatalf("ops = %v, want none", ops)
	}
}

// An unknown algorithm cannot arrive through WithAlgorithm, which rejects one
// where it is passed. Reaching the dispatch with one is a broken invariant, so
// it panics rather than returning an error no caller could have acted on.
func TestRunDispatchPanicsOnUnknownAlgorithm(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("run with unknown algorithm: want panic, got none")
		}
		if want := "similar: unknown algorithm 99"; r != want {
			t.Fatalf("panic = %v, want %q", r, want)
		}
	}()
	_ = run(context.Background(), unknownAlg, NewCapture(), []int{1}, 0, 1, []int{2}, 0, 1)
}

func TestRunDispatchDiffsSubRanges(t *testing.T) {
	old := []int{9, 1, 2, 3, 9}
	new := []int{8, 1, 2, 3, 8}

	capture := NewCapture()
	if err := run(context.Background(), Myers, capture, old, 1, 4, new, 1, 4); err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []DiffOp{{Tag: Equal, OldIndex: 1, NewIndex: 1, OldLen: 3, NewLen: 3}}
	if !reflect.DeepEqual(capture.Ops(), want) {
		t.Fatalf("ops = %v, want %v", capture.Ops(), want)
	}
}

// failingHook fails on its first Equal callback, standing in for any hook whose
// callback errors mid-diff.
type failingHook struct {
	NoopHook
	err error
}

func (h *failingHook) Equal(oldIndex, newIndex, length int) error { return h.err }

func TestRunDispatchPropagatesHookError(t *testing.T) {
	boom := errors.New("boom")
	err := run(context.Background(), Myers, &failingHook{err: boom},
		[]int{1, 2}, 0, 2, []int{1, 2}, 0, 2)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

func TestCaptureOpsCancelledContextIsNotAnError(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	old := make([]int, 2000)
	new := make([]int, 2000)
	for i := range old {
		old[i] = i
		new[i] = i * 2
	}

	// An expired deadline yields an approximate but valid script, not an error.
	ops := captureOps(ctx, Myers, old, new)
	if len(ops) == 0 {
		t.Fatal("ops = none, want a valid (possibly approximate) script")
	}
}

// The heuristics in internal/algorithms — the disjoint fast path and the
// small-side-exact walks — are exactness-preserving, so their output is
// indistinguishable from full search by inspection. What was never covered is
// what happens to that output afterwards: every heuristic emits its callbacks
// straight into whatever hook it was handed, and the library always hands it
// Compact(Replace(Capture)). These tests drive heuristic-shaped inputs through
// that stack and check the script that comes out the far side.
//
// Whether a given heuristic actually fired is asserted where it can be observed
// directly, next to the heuristic itself — see internal/algorithms.

// applyOps rebuilds the new sequence from old and an op script.
func applyOps[T comparable](old, new []T, ops []DiffOp) []T {
	out := make([]T, 0, len(new))
	for _, op := range ops {
		switch op.Tag {
		case Equal:
			out = append(out, old[op.OldIndex:op.OldIndex+op.OldLen]...)
		case Insert, Replace:
			out = append(out, new[op.NewIndex:op.NewIndex+op.NewLen]...)
		}
	}
	return out
}

// scriptCost is the number of items the script adds or removes.
func scriptCost(ops []DiffOp) int {
	cost := 0
	for _, op := range ops {
		switch op.Tag {
		case Delete:
			cost += op.OldLen
		case Insert:
			cost += op.NewLen
		case Replace:
			cost += op.OldLen + op.NewLen
		}
	}
	return cost
}

// assertScriptCovers checks the three invariants every script off the standard
// hook stack must hold: it rebuilds new from old, its ops are cursor-contiguous,
// and together they span both sequences exactly.
func assertScriptCovers[T comparable](t *testing.T, old, new []T, ops []DiffOp) {
	t.Helper()

	if got := applyOps(old, new, ops); !slices.Equal(got, new) {
		t.Fatalf("script does not rebuild new: got %d items, want %d", len(got), len(new))
	}

	oldCursor, newCursor := 0, 0
	for i, op := range ops {
		os, oe := op.OldRange()
		ns, ne := op.NewRange()
		if os != oldCursor || ns != newCursor {
			t.Fatalf("op %d (%v) starts at (%d,%d), want (%d,%d)", i, op.Tag, os, ns, oldCursor, newCursor)
		}
		oldCursor, newCursor = oe, ne
	}
	if oldCursor != len(old) || newCursor != len(new) {
		t.Fatalf("script covers (%d,%d), want (%d,%d)", oldCursor, newCursor, len(old), len(new))
	}
}

// TestDisjointFastPathThroughHookStack covers two large ranges sharing nothing.
// The fast path emits one Delete immediately followed by one Insert, which is
// exactly the pair ReplaceHook exists to coalesce — so the caller sees a single
// Replace spanning both sequences.
func TestDisjointFastPathThroughHookStack(t *testing.T) {
	const n = 512
	old := make([]int, n)
	new := make([]int, n)
	for i := range old {
		old[i] = i
		new[i] = i + n
	}

	ops := captureOps(context.Background(), Myers, old, new)

	assertScriptCovers(t, old, new, ops)

	// A caller-visible Replace is always this synthesis; the core never emits one.
	want := []DiffOp{{Tag: Replace, OldIndex: 0, NewIndex: 0, OldLen: n, NewLen: n}}
	if !reflect.DeepEqual(ops, want) {
		t.Fatalf("ops = %v, want %v", ops, want)
	}
	if got, want := scriptCost(ops), 2*n; got != want {
		t.Fatalf("script cost = %d, want %d", got, want)
	}
}

// TestSmallSideExactThroughHookStack covers a long sequence against a single
// item drawn from its middle. The small-side-exact walk emits deletes either
// side of one equal run; no Delete is adjacent to an Insert, so nothing is
// coalesced and the caller sees the deletes unmerged.
func TestSmallSideExactThroughHookStack(t *testing.T) {
	const n = 1000
	old := make([]int, n)
	for i := range old {
		old[i] = i
	}
	new := []int{500}

	ops := captureOps(context.Background(), Myers, old, new)

	assertScriptCovers(t, old, new, ops)

	want := []DiffOp{
		{Tag: Delete, OldIndex: 0, NewIndex: 0, OldLen: 500},
		{Tag: Equal, OldIndex: 500, NewIndex: 0, OldLen: 1, NewLen: 1},
		{Tag: Delete, OldIndex: 501, NewIndex: 1, OldLen: 499},
	}
	if !reflect.DeepEqual(ops, want) {
		t.Fatalf("ops = %v, want %v", ops, want)
	}
	if got, want := scriptCost(ops), n-1; got != want {
		t.Fatalf("script cost = %d, want %d", got, want)
	}
}

// captureRatio skips the standard hook stack on the claim that compaction and
// replace-coalescing preserve the total length of the Equal spans. That claim is
// the whole basis for the cheaper path, so it is checked directly, over random
// pairs across the shapes the heuristics and the recursion each handle.
func TestCaptureRatioMatchesTheStandardStack(t *testing.T) {
	r := rand.New(rand.NewSource(20260803))
	alphabets := []string{"ab", "abcdefgh", "abcdefghijklmnopqrstuvwxyz"}

	for _, alphabet := range alphabets {
		for _, size := range []int{0, 1, 2, 7, 40, 200} {
			for i := 0; i < 120; i++ {
				mk := func() []string {
					n := r.Intn(size + 1)
					s := make([]string, n)
					for j := range s {
						s[j] = string(alphabet[r.Intn(len(alphabet))])
					}
					return s
				}
				old, new := mk(), mk()

				viaStack := DiffRatio(captureOps(context.Background(), Myers, old, new), len(old), len(new))
				viaCounter := captureRatio(context.Background(), Myers, old, new)
				if viaStack != viaCounter {
					t.Fatalf("alphabet %q size %d: stack ratio %v, counter ratio %v\nold=%q\nnew=%q",
						alphabet, size, viaStack, viaCounter, old, new)
				}
			}
		}
	}
}

// The same claim at the two shapes that skip the recursion entirely: the
// disjoint fast path and the small-side-exact walk.
func TestCaptureRatioMatchesTheStandardStackForHeuristicShapes(t *testing.T) {
	const n = 512
	disjointOld := make([]int, n)
	disjointNew := make([]int, n)
	for i := range disjointOld {
		disjointOld[i] = i
		disjointNew[i] = i + n
	}

	longSide := make([]int, 1000)
	for i := range longSide {
		longSide[i] = i
	}

	cases := []struct {
		name     string
		old, new []int
	}{
		{"disjoint", disjointOld, disjointNew},
		{"small side exact", longSide, []int{500}},
		{"identical", longSide, longSide},
		{"both empty", nil, nil},
		{"old empty", nil, []int{1, 2, 3}},
		{"new empty", []int{1, 2, 3}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			viaStack := DiffRatio(captureOps(context.Background(), Myers, c.old, c.new), len(c.old), len(c.new))
			viaCounter := captureRatio(context.Background(), Myers, c.old, c.new)
			if viaStack != viaCounter {
				t.Fatalf("stack ratio %v, counter ratio %v", viaStack, viaCounter)
			}
		})
	}
}
