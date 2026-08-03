package algorithms

import (
	"context"
	"testing"
	"time"
)

// captureLCSLimits runs the LCS core with a gate set the test chose, which is
// how the table gate's edge is driven without allocating a table large enough
// to cross the production one.
func captureLCSLimits[T comparable](old, new []T, lim limits) []capturedOp {
	c := newCapture()
	if err := diffLCS(c, old, 0, len(old), new, 0, len(new), lim); err != nil {
		panic(err)
	}
	return c.Ops()
}

// --- Ported Rust behavioral fixtures, judged by the oracle ---

// TestLCSTable ports the Rust `test_table` fixture. The crate stores only the
// non-zero cells — (0,0), (1,0) and (2,0), all 1 — so the assertion here is that
// same table with its zeros written out, including the sentinel row and column.
func TestLCSTable(t *testing.T) {
	old := []int{2, 3}
	new := []int{0, 1, 2}

	table, stride := makeLCSTable(old, 0, len(old), new, 0, len(new), noDeadline)
	if table == nil {
		t.Fatal("table was declined")
	}
	if stride != len(old)+1 {
		t.Fatalf("stride = %d, want %d", stride, len(old)+1)
	}

	want := []int32{
		1, 0, 0,
		1, 0, 0,
		1, 0, 0,
		0, 0, 0,
	}
	if !slicesEqual(table, want) {
		t.Fatalf("table = %v, want %v", table, want)
	}
}

// TestLCSDiff ports the Rust `test_diff` fixture.
func TestLCSDiff(t *testing.T) {
	a := []int{0, 1, 2, 3, 4}
	b := []int{0, 1, 2, 9, 4}
	assertInvariants(t, a, b, captureLCS(a, b))
}

// TestLCSContiguous ports the Rust `test_contiguous` fixture.
func TestLCSContiguous(t *testing.T) {
	a := []int{0, 1, 2, 3, 4, 4, 4, 5}
	b := []int{0, 1, 2, 8, 9, 4, 4, 7}
	assertInvariants(t, a, b, captureLCS(a, b))
}

// TestLCSPat ports the Rust `test_pat` fixture.
func TestLCSPat(t *testing.T) {
	a := []int{0, 1, 3, 4, 5}
	b := []int{0, 1, 4, 5, 8, 9}
	assertInvariants(t, a, b, captureLCS(a, b))
}

// TestLCSSame ports the Rust `test_same` fixture: identical ranges collapse to
// one Equal.
func TestLCSSame(t *testing.T) {
	a := []int{0, 1, 2, 3, 4, 4, 4, 5}
	b := []int{0, 1, 2, 3, 4, 4, 4, 5}
	ops := captureLCS(a, b)
	assertInvariants(t, a, b, ops)
	want := []capturedOp{{Tag: tagEqual, OldIndex: 0, NewIndex: 0, OldLen: 8, NewLen: 8}}
	assertOps(t, ops, want)
}

// TestLCSIssue44SwappedRegression ports the Rust `test_issue44_swapped_regression`
// fixture, which pins the operations exactly — including the two adjacent
// single-element Equals the table walk emits, which only a ReplaceHook above it
// would merge.
func TestLCSIssue44SwappedRegression(t *testing.T) {
	a := []int{0, 1, 4, 5, 8, 9}
	b := []int{0, 1, 3, 4, 5}

	ops := captureLCS(a, b)
	want := []capturedOp{
		{Tag: tagEqual, OldIndex: 0, NewIndex: 0, OldLen: 2, NewLen: 2},
		{Tag: tagInsert, OldIndex: 2, NewIndex: 2, NewLen: 1},
		{Tag: tagEqual, OldIndex: 2, NewIndex: 3, OldLen: 1, NewLen: 1},
		{Tag: tagEqual, OldIndex: 3, NewIndex: 4, OldLen: 1, NewLen: 1},
		{Tag: tagDelete, OldIndex: 4, NewIndex: 5, OldLen: 2},
	}
	assertOps(t, ops, want)
	assertInvariants(t, a, b, ops)
}

// TestLCSSubRangeRegression ports the Rust `test_subrange_regression` fixture:
// the same script as above, shifted into a window of two larger sequences, in
// absolute coordinates.
func TestLCSSubRangeRegression(t *testing.T) {
	a := []int{99, 0, 1, 4, 5, 8, 9, 88}
	b := []int{77, 0, 1, 3, 4, 5, 66}

	c := newCapture()
	if err := DiffLCS(c, a, 1, 7, b, 1, 6); err != nil {
		t.Fatal(err)
	}
	want := []capturedOp{
		{Tag: tagEqual, OldIndex: 1, NewIndex: 1, OldLen: 2, NewLen: 2},
		{Tag: tagInsert, OldIndex: 3, NewIndex: 3, NewLen: 1},
		{Tag: tagEqual, OldIndex: 3, NewIndex: 4, OldLen: 1, NewLen: 1},
		{Tag: tagEqual, OldIndex: 4, NewIndex: 5, OldLen: 1, NewLen: 1},
		{Tag: tagDelete, OldIndex: 5, NewIndex: 6, OldLen: 2},
	}
	assertOps(t, c.Ops(), want)
}

// TestLCSIdenticalSubRange covers the one divergence from the crate on this
// path: its all-equal shortcut emits equal(0, 0, len), which reports the wrong
// span for a window that does not start at zero.
func TestLCSIdenticalSubRange(t *testing.T) {
	a := []int{99, 1, 2, 3, 88}
	b := []int{77, 1, 2, 3, 66}

	c := newCapture()
	if err := DiffLCS(c, a, 1, 4, b, 1, 4); err != nil {
		t.Fatal(err)
	}
	want := []capturedOp{{Tag: tagEqual, OldIndex: 1, NewIndex: 1, OldLen: 3, NewLen: 3}}
	assertOps(t, c.Ops(), want)
}

// TestLCSBadRangeRegression ports the Rust `test_bad_range_regression` fixture:
// the common prefix consumes all of old, leaving a pure insert.
func TestLCSBadRangeRegression(t *testing.T) {
	a := []int{0}
	b := []int{0, 0}

	ops := captureLCS(a, b)
	want := []capturedOp{
		{Tag: tagEqual, OldIndex: 0, NewIndex: 0, OldLen: 1, NewLen: 1},
		{Tag: tagInsert, OldIndex: 1, NewIndex: 1, NewLen: 1},
	}
	assertOps(t, ops, want)
	assertInvariants(t, a, b, ops)
}

// TestLCSFinishCalled ports the Rust `test_finish_called` fixture. The empty
// case is the second divergence: the crate emits a zero-length delete for it and
// this package emits nothing, but Finish is called either way.
func TestLCSFinishCalled(t *testing.T) {
	cases := []struct {
		name     string
		old, new []int
	}{
		{"differ", []int{1, 2}, []int{1, 2, 3}},
		{"identical", []int{1, 2}, []int{1, 2}},
		{"empty", []int{}, []int{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &finishRecorder{}
			if err := DiffLCS(f, tc.old, 0, len(tc.old), tc.new, 0, len(tc.new)); err != nil {
				t.Fatal(err)
			}
			if !f.finished {
				t.Fatal("finish was not called")
			}
		})
	}
}

func TestLCSEmptyRangesEmitNothing(t *testing.T) {
	if ops := captureLCS([]int{}, []int{}); len(ops) != 0 {
		t.Fatalf("ops = %v, want none", ops)
	}
}

// --- Hook errors abort and propagate ---

func TestLCSHookErrorPropagates(t *testing.T) {
	a := []int{0, 1, 2, 3, 4}
	b := []int{9, 8, 7, 6, 5}
	h := &errAfter{remaining: 0}
	if err := DiffLCS(h, a, 0, len(a), b, 0, len(b)); err != errBoom {
		t.Fatalf("err = %v, want boom", err)
	}
}

// --- Deadline behavior ---

func TestLCSDeadlineReachedYieldsValidScript(t *testing.T) {
	a := make([]int, 100)
	b := make([]int, 100)
	for i := range a {
		a[i] = i
		b[i] = i
	}
	b[10], b[25], b[50] = 99, 99, 99

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	ops := captureLCSDeadline(ctx, a, b)
	if got := reconstruct(a, b, ops); !slicesEqual(got, b) {
		t.Fatalf("deadline-hit reconstruct = %v, want %v", got, b)
	}
}

func TestLCSContextCancellationYieldsValidScript(t *testing.T) {
	a := make([]int, 200)
	b := make([]int, 200)
	for i := range a {
		a[i] = i
		b[i] = i + 1 // fully shifted, forces the table
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	ops := captureLCSDeadline(ctx, a, b)
	if got := reconstruct(a, b, ops); !slicesEqual(got, b) {
		t.Fatal("cancelled reconstruct did not rebuild new")
	}
}

// --- The table gate ---

// TestLCSTableGate drives lcsTableMaxWork's edge. At the cap the table is built
// and the script is minimal; one cell over it the table is declined and the
// middle is replaced wholesale — still reconstructing, no longer minimal, and
// emitted exactly once (the crate emits that pair twice, which reconstructs new
// twice over).
func TestLCSTableGate(t *testing.T) {
	old := []int{1, 2, 3, 4, 5}
	new := []int{1, 9, 3, 8, 5}
	// One element of common prefix and one of common suffix leave a middle of
	// old[1:4] against new[1:4], so the table is four rows of four.
	const cells = 4 * 4

	t.Run("at the cap", func(t *testing.T) {
		lim := noDeadline
		lim.lcsTableMaxWork = cells
		assertInvariants(t, old, new, captureLCSLimits(old, new, lim))
	})

	t.Run("over the cap", func(t *testing.T) {
		lim := noDeadline
		lim.lcsTableMaxWork = cells - 1
		ops := captureLCSLimits(old, new, lim)
		if got := reconstruct(old, new, ops); !slicesEqual(got, new) {
			t.Fatalf("declined-table reconstruct = %v, want %v", got, new)
		}
		checkContiguous(t, old, new, ops)
		want := []capturedOp{
			{Tag: tagEqual, OldIndex: 0, NewIndex: 0, OldLen: 1, NewLen: 1},
			{Tag: tagDelete, OldIndex: 1, NewIndex: 1, OldLen: 3},
			{Tag: tagInsert, OldIndex: 4, NewIndex: 1, NewLen: 3},
			{Tag: tagEqual, OldIndex: 4, NewIndex: 4, OldLen: 1, NewLen: 1},
		}
		assertOps(t, ops, want)
	})
}

func TestMakeLCSTableDeclinesOverTheGate(t *testing.T) {
	lim := noDeadline
	lim.lcsTableMaxWork = 3 // a 1x1 middle needs 2*2
	old := []int{1, 2}
	new := []int{3, 4}
	if table, _ := makeLCSTable(old, 0, len(old), new, 0, len(new), lim); table != nil {
		t.Fatal("table was built over the gate")
	}
	if table, _ := makeLCSTable(old, 0, len(old), new, 0, len(new), expired()); table != nil {
		t.Fatal("table was built past the deadline")
	}
}
