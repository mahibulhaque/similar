package similar

import (
	"context"
	"reflect"
	"slices"
	"testing"
)

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

	ops, err := captureOps(context.Background(), Myers, old, new)
	if err != nil {
		t.Fatalf("captureOps: %v", err)
	}

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

	ops, err := captureOps(context.Background(), Myers, old, new)
	if err != nil {
		t.Fatalf("captureOps: %v", err)
	}

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
