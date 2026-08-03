package algorithms

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// The shared test harness: the capture substrate every test records through,
// the gate sets they run against, and the oracle that judges what was recorded.

// --- Gate sets ---

// noDeadline is the production gate set with no time limit and no cancellation
// — what fromContext yields for a background context.
var noDeadline = fromContext(context.Background())

// expired returns the production gates with a wall-clock deadline already past.
func expired() limits {
	lim := fromContext(context.Background())
	lim.time = time.Now().Add(-time.Second)
	return lim
}

// --- Capture substrate ---

// The tests judge Myers output as a list of operations, but this package never
// builds one: it only calls diffHook callbacks. These types are that
// materialization, kept test-local so the unit tests depend on the shape of the
// callbacks under test rather than on whichever operation type a consumer
// happens to assemble from them.

// opTag identifies the kind of a capturedOp.
type opTag int

const (
	tagEqual opTag = iota
	tagDelete
	tagInsert
)

// String keeps failure messages readable, naming the tag rather than its ordinal.
func (t opTag) String() string {
	switch t {
	case tagEqual:
		return "equal"
	case tagDelete:
		return "delete"
	case tagInsert:
		return "insert"
	default:
		return fmt.Sprintf("opTag(%d)", int(t))
	}
}

// capturedOp is one callback recorded as a tagged span. For an Equal, OldLen and
// NewLen are both the span length; a Delete leaves NewLen zero and an Insert
// leaves OldLen zero, so the ranges are computed the same way for every tag.
type capturedOp struct {
	Tag      opTag
	OldIndex int
	NewIndex int
	OldLen   int
	NewLen   int
}

// OldRange returns the half-open range the op covers in the old sequence.
func (o capturedOp) OldRange() (start, end int) { return o.OldIndex, o.OldIndex + o.OldLen }

// NewRange returns the half-open range the op covers in the new sequence.
func (o capturedOp) NewRange() (start, end int) { return o.NewIndex, o.NewIndex + o.NewLen }

// capture is a diffHook that records every callback in order.
type capture struct {
	ops []capturedOp
}

func newCapture() *capture { return &capture{} }

// Ops returns the recorded operations.
func (c *capture) Ops() []capturedOp { return c.ops }

func (c *capture) Equal(oldIndex, newIndex, length int) error {
	c.ops = append(c.ops, capturedOp{tagEqual, oldIndex, newIndex, length, length})
	return nil
}

func (c *capture) Delete(oldIndex, oldLen, newIndex int) error {
	c.ops = append(c.ops, capturedOp{Tag: tagDelete, OldIndex: oldIndex, NewIndex: newIndex, OldLen: oldLen})
	return nil
}

func (c *capture) Insert(oldIndex, newIndex, newLen int) error {
	c.ops = append(c.ops, capturedOp{Tag: tagInsert, OldIndex: oldIndex, NewIndex: newIndex, NewLen: newLen})
	return nil
}

func (c *capture) Finish() error { return nil }

// noopHook is an embeddable diffHook whose callbacks all return nil, so a test
// hook can override only the callback it cares about.
type noopHook struct{}

func (noopHook) Equal(oldIndex, newIndex, length int) error  { return nil }
func (noopHook) Delete(oldIndex, oldLen, newIndex int) error { return nil }
func (noopHook) Insert(oldIndex, newIndex, newLen int) error { return nil }
func (noopHook) Finish() error                               { return nil }

var (
	_ diffHook = (*capture)(nil)
	_ diffHook = noopHook{}
)

// --- Oracle ---

// The oracle: independent, brute-force helpers used to judge Myers output
// without hand-authored expected operations. Each is self-tested below before
// it is trusted.

// bruteLCS is an O(N*M) longest-common-subsequence length DP.
func bruteLCS[T comparable](old, new []T) int {
	n, m := len(old), len(new)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if old[i] == new[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else {
				dp[i][j] = max(dp[i+1][j], dp[i][j+1])
			}
		}
	}
	return dp[0][0]
}

// reconstruct applies ops to old and returns the rebuilt new sequence.
func reconstruct[T comparable](old, new []T, ops []capturedOp) []T {
	out := make([]T, 0, len(new))
	for _, op := range ops {
		switch op.Tag {
		case tagEqual:
			out = append(out, old[op.OldIndex:op.OldIndex+op.OldLen]...)
		case tagDelete:
			// nothing produced
		case tagInsert:
			out = append(out, new[op.NewIndex:op.NewIndex+op.NewLen]...)
		}
	}
	return out
}

// editCost sums the deleted and inserted lengths.
func editCost(ops []capturedOp) int {
	cost := 0
	for _, op := range ops {
		switch op.Tag {
		case tagDelete:
			cost += op.OldLen
		case tagInsert:
			cost += op.NewLen
		}
	}
	return cost
}

// captureMyers runs the raw Myers core over the whole slices with a bare
// Capture hook — equal/delete/insert only, with no Compact stage above it.
func captureMyers[T comparable](old, new []T) []capturedOp {
	c := newCapture()
	if err := DiffMyers(c, old, 0, len(old), new, 0, len(new)); err != nil {
		panic(err)
	}
	return c.Ops()
}

func captureMyersDeadline[T comparable](ctx context.Context, old, new []T) []capturedOp {
	c := newCapture()
	if err := DiffMyersDeadline(ctx, c, old, 0, len(old), new, 0, len(new)); err != nil {
		panic(err)
	}
	return c.Ops()
}

func slicesEqual[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// checkContiguous verifies operations are contiguous, non-overlapping, and
// fully cover both sequences. Each side is checked with its own cursor, since
// a raw script may emit a Delete and an Insert both anchored at the same old
// position (the standard Myers base case) — those interleave, but every op
// that consumes old and every op that produces new is individually contiguous.
func checkContiguous[T comparable](t *testing.T, old, new []T, ops []capturedOp) {
	t.Helper()
	oldCursor, newCursor := 0, 0
	for i, op := range ops {
		os, oe := op.OldRange()
		ns, ne := op.NewRange()
		switch op.Tag {
		case tagEqual, tagDelete:
			if os != oldCursor {
				t.Fatalf("op %d (%v): old start %d, expected %d", i, op.Tag, os, oldCursor)
			}
			oldCursor = oe
		}
		switch op.Tag {
		case tagEqual, tagInsert:
			if ns != newCursor {
				t.Fatalf("op %d (%v): new start %d, expected %d", i, op.Tag, ns, newCursor)
			}
			newCursor = ne
		}
	}
	if oldCursor != len(old) {
		t.Fatalf("ops cover old up to %d, want %d", oldCursor, len(old))
	}
	if newCursor != len(new) {
		t.Fatalf("ops cover new up to %d, want %d", newCursor, len(new))
	}
}

// assertInvariants checks the two load-bearing behaviors plus coverage.
func assertInvariants[T comparable](t *testing.T, old, new []T, ops []capturedOp) {
	t.Helper()
	got := reconstruct(old, new, ops)
	if !slicesEqual(got, new) {
		t.Fatalf("reconstruct(old, ops) = %v, want %v (old=%v)", got, new, old)
	}
	wantCost := len(old) + len(new) - 2*bruteLCS(old, new)
	if c := editCost(ops); c != wantCost {
		t.Fatalf("editCost = %d, want %d (minimal); old=%v new=%v ops=%v", c, wantCost, old, new, ops)
	}
	checkContiguous(t, old, new, ops)
}

func TestOracleSelfCheck(t *testing.T) {
	if got := bruteLCS([]int{1, 2, 3}, []int{1, 2, 3}); got != 3 {
		t.Fatalf("bruteLCS identical = %d, want 3", got)
	}
	if got := bruteLCS([]int{1, 2, 3}, []int{4, 5, 6}); got != 0 {
		t.Fatalf("bruteLCS disjoint = %d, want 0", got)
	}
	if got := bruteLCS([]int{1, 2, 3, 4}, []int{1, 3, 4}); got != 3 {
		t.Fatalf("bruteLCS subseq = %d, want 3", got)
	}
	if got := bruteLCS([]int{}, []int{1, 2}); got != 0 {
		t.Fatalf("bruteLCS empty = %d, want 0", got)
	}

	old := []int{1, 2, 3}
	new := []int{1, 9, 3}
	ops := []capturedOp{
		{Tag: tagEqual, OldIndex: 0, NewIndex: 0, OldLen: 1, NewLen: 1},
		{Tag: tagDelete, OldIndex: 1, NewIndex: 1, OldLen: 1},
		{Tag: tagInsert, OldIndex: 2, NewIndex: 1, NewLen: 1},
		{Tag: tagEqual, OldIndex: 2, NewIndex: 2, OldLen: 1, NewLen: 1},
	}
	if got := reconstruct(old, new, ops); !slicesEqual(got, new) {
		t.Fatalf("reconstruct self-check = %v, want %v", got, new)
	}
	if got := editCost(ops); got != 2 {
		t.Fatalf("editCost self-check = %d, want 2", got)
	}
}
