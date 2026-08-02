package algorithms

import (
	"context"
	"testing"
	"time"
)

// --- Ported Rust behavioral fixtures, judged by the oracle ---

func TestFindMiddleSnake(t *testing.T) {
	a := []byte("ABCABBA")
	b := []byte("CBABAC")
	md := maxD(len(a), len(b))
	vf := newReachVector(md)
	vb := newReachVector(md)
	x, y, ok := findMiddleSnake(a, 0, len(a), b, 0, len(b), vf, vb, noDeadline)
	if !ok {
		t.Fatal("no middle snake found")
	}
	if x != 4 || y != 1 {
		t.Fatalf("middle snake = (%d, %d), want (4, 1)", x, y)
	}
}

func TestDiff(t *testing.T) {
	a := []int{0, 1, 2, 3, 4}
	b := []int{0, 1, 2, 9, 4}
	ops := captureMyers(a, b)
	assertInvariants(t, a, b, ops)
}

func TestContiguous(t *testing.T) {
	a := []int{0, 1, 2, 3, 4, 4, 4, 5}
	b := []int{0, 1, 2, 8, 9, 4, 4, 7}
	ops := captureMyers(a, b)
	assertInvariants(t, a, b, ops)
}

func TestPat(t *testing.T) {
	a := []int{0, 1, 3, 4, 5}
	b := []int{0, 1, 4, 5, 8, 9}
	ops := captureMyers(a, b)
	assertInvariants(t, a, b, ops)
}

func TestFrontAnchorRegressionsStayExact(t *testing.T) {
	t.Run("large append shift", func(t *testing.T) {
		old := []int{0, 1}
		for i := 10; i < 106; i++ {
			old = append(old, i)
		}
		new := []int{1, 2}
		for i := 10; i < 106; i++ {
			new = append(new, i)
		}
		for i := 1000; i < 1098; i++ {
			new = append(new, i)
		}
		ops := captureMyers(old, new)
		assertInvariants(t, old, new, ops)

		equalLen := 0
		for _, op := range ops {
			if op.Tag == tagEqual {
				equalLen += op.OldLen
			}
		}
		if equalLen != 97 {
			t.Fatalf("equal length = %d, want 97", equalLen)
		}
		if c := editCost(ops); c != 100 {
			t.Fatalf("edit cost = %d, want 100", c)
		}
	})

	t.Run("i%%2 repetitive pattern", func(t *testing.T) {
		old := make([]int, 99)
		for i := range old {
			old[i] = i % 2
		}
		new := make([]int, 0, len(old)*2)
		new = append(new, old[1:]...)
		new = append(new, old...)
		new = append(new, 0)

		ops := captureMyers(old, new)
		assertInvariants(t, old, new, ops)
		if c := editCost(ops); c != 99 {
			t.Fatalf("edit cost = %d, want 99 (== lcs cost)", c)
		}
	})
}

// --- Deadline behavior ---

func TestDeadlineReachedYieldsValidScript(t *testing.T) {
	a := make([]int, 100)
	b := make([]int, 100)
	for i := range a {
		a[i] = i
		b[i] = i
	}
	b[10], b[25], b[50] = 99, 99, 99

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	ops := captureMyersDeadline(ctx, a, b)
	if got := reconstruct(a, b, ops); !slicesEqual(got, b) {
		t.Fatalf("deadline-hit reconstruct = %v, want %v", got, b)
	}
}

func TestContextCancellationYieldsValidScript(t *testing.T) {
	a := make([]int, 200)
	b := make([]int, 200)
	for i := range a {
		a[i] = i
		b[i] = i + 1 // fully shifted, forces search
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	ops := captureMyersDeadline(ctx, a, b)
	if got := reconstruct(a, b, ops); !slicesEqual(got, b) {
		t.Fatal("cancelled reconstruct did not rebuild new")
	}
}

// TestHeuristicDeadlineGuards asserts the heuristics do no work and emit
// nothing once the deadline is already exceeded.
func TestHeuristicDeadlineGuards(t *testing.T) {
	past := expired()

	t.Run("small-side-exact bails", func(t *testing.T) {
		old := make([]byte, defaultSmallSideExactMax)
		new := make([]byte, defaultSmallSideExactMaxWork/defaultSmallSideExactMax)
		for i := range old {
			old[i] = 1
		}
		for i := range new {
			new[i] = 1
		}
		c := newCapture()
		used, err := maybeEmitSmallSideExact(c, old, 0, len(old), new, 0, len(new), past)
		if err != nil {
			t.Fatal(err)
		}
		if used {
			t.Fatal("expected not used when deadline exceeded")
		}
		if len(c.Ops()) != 0 {
			t.Fatalf("expected no ops, got %v", c.Ops())
		}
	})

	t.Run("front-anchor bails", func(t *testing.T) {
		old := make([]byte, 4096)
		new := make([]byte, 8200)
		for i := range old {
			old[i] = 1
		}
		for i := range new {
			new[i] = 1
		}
		c := newCapture()
		os, ns, err := tryEmitFrontAnchor(c, old, 0, len(old), new, 0, len(new), past)
		if err != nil {
			t.Fatal(err)
		}
		if os != 0 || ns != 0 {
			t.Fatalf("ranges advanced to (%d,%d), want (0,0)", os, ns)
		}
		if len(c.Ops()) != 0 {
			t.Fatalf("expected no ops, got %v", c.Ops())
		}

		// With a live deadline the anchor scan does run.
		c2 := newCapture()
		os2, ns2, err := tryEmitFrontAnchor(c2, old, 0, len(old), new, 0, len(new), noDeadline)
		if err != nil {
			t.Fatal(err)
		}
		if os2 == 0 && ns2 == 0 && len(c2.Ops()) == 0 {
			t.Fatal("expected anchor scan to advance ranges with a live deadline")
		}
	})
}

// --- Finish is always called, including for empty inputs ---

type finishRecorder struct {
	noopHook
	finished bool
}

func (f *finishRecorder) Finish() error {
	f.finished = true
	return nil
}

func TestFinishCalled(t *testing.T) {
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
			if err := Diff(f, tc.old, 0, len(tc.old), tc.new, 0, len(tc.new)); err != nil {
				t.Fatal(err)
			}
			if !f.finished {
				t.Fatal("finish was not called")
			}
		})
	}
}

// --- Hook errors abort and propagate ---

type errAfter struct {
	noopHook
	remaining int
}

var errBoom = &boomError{}

type boomError struct{}

func (*boomError) Error() string { return "boom" }

func (e *errAfter) Equal(int, int, int) error  { return e.tick() }
func (e *errAfter) Delete(int, int, int) error { return e.tick() }
func (e *errAfter) Insert(int, int, int) error { return e.tick() }
func (e *errAfter) tick() error {
	if e.remaining <= 0 {
		return errBoom
	}
	e.remaining--
	return nil
}

func TestHookErrorPropagates(t *testing.T) {
	a := []int{0, 1, 2, 3, 4}
	b := []int{9, 8, 7, 6, 5}
	h := &errAfter{remaining: 0}
	err := Diff(h, a, 0, len(a), b, 0, len(b))
	if err != errBoom {
		t.Fatalf("err = %v, want boom", err)
	}
}

// --- Invariant matrix over the edge-case shapes ---

func TestInvariantMatrix(t *testing.T) {
	rep := func(v, n int) []int {
		s := make([]int, n)
		for i := range s {
			s[i] = v
		}
		return s
	}
	seq := func(start, n int) []int {
		s := make([]int, n)
		for i := range s {
			s[i] = start + i
		}
		return s
	}
	reversed := func(s []int) []int {
		r := make([]int, len(s))
		for i := range s {
			r[i] = s[len(s)-1-i]
		}
		return r
	}
	alt := func(n int) []int {
		s := make([]int, n)
		for i := range s {
			s[i] = i % 2
		}
		return s
	}

	cases := []struct {
		name     string
		old, new []int
	}{
		{"both empty", nil, nil},
		{"pure insert", nil, []int{1, 2, 3}},
		{"pure delete", []int{1, 2, 3}, nil},
		{"identical", seq(0, 20), seq(0, 20)},
		{"fully disjoint small", []int{1, 2, 3}, []int{4, 5, 6}},
		{"single equal", []int{7}, []int{7}},
		{"single differ", []int{7}, []int{8}},
		{"common prefix only", []int{1, 2, 3, 4}, []int{1, 2, 9, 8}},
		{"common suffix only", []int{9, 8, 3, 4}, []int{1, 2, 3, 4}},
		{"prefix+suffix changed middle", []int{1, 2, 3, 4, 5}, []int{1, 2, 9, 4, 5}},
		{"single differing middle", []int{1, 2, 3}, []int{1, 9, 3}},
		{"reversed", seq(0, 15), reversed(seq(0, 15))},
		{"interleaved", []int{1, 2, 3, 4, 5, 6}, []int{1, 9, 3, 8, 5, 7}},
		{"all identical repeated", rep(5, 30), rep(5, 30)},
		{"repeated with edits", rep(5, 30), append(append(rep(5, 15), 9), rep(5, 14)...)},
		{"alt pattern shifted", alt(40), append([]int{1}, alt(40)...)},
		{"large disjoint (fast path)", seq(0, 600), seq(10000, 600)},
		{"tiny vs large (small side)", []int{1, 2, 3}, seq(5000, 700)},
		{"large vs tiny (small side)", seq(5000, 700), []int{1, 2, 3}},
		{"unbalanced prepend (anchor)", seq(0, 300), append(seq(9000, 5), seq(0, 300)...)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := captureMyers(tc.old, tc.new)
			assertInvariants(t, tc.old, tc.new, ops)
		})
	}
}

// --- Heuristic threshold boundaries at N-1 / N / N+1 ---

func TestHeuristicBoundaries(t *testing.T) {
	seq := func(start, n int) []int {
		s := make([]int, n)
		for i := range s {
			s[i] = start + i
		}
		return s
	}
	consts := []int{
		defaultSmallSideExactMax,      // 64
		defaultSmallSideExactMinLarge, // 512
		defaultDisjointFastPathMinLen, // 512
		defaultFrontAnchorMinCommon,   // 96
	}
	for _, base := range consts {
		for _, delta := range []int{-1, 0, 1} {
			n := base + delta
			if n < 1 {
				continue
			}
			t.Run("", func(t *testing.T) {
				// disjoint-ish and overlapping shapes at the boundary size
				old := seq(0, n)
				new := seq(n/2, n)
				assertInvariants(t, old, new, captureMyers(old, new))

				old2 := seq(0, 3)
				new2 := seq(100000, n)
				assertInvariants(t, old2, new2, captureMyers(old2, new2))
			})
		}
	}
}

// --- Non-zero start (sub-range) diffing ---

func TestSubRangeDiff(t *testing.T) {
	// Full sequences; diff only a window with non-zero start.
	old := []int{100, 101, 1, 2, 3, 4, 200}
	new := []int{100, 101, 1, 9, 3, 4, 200}
	c := newCapture()
	if err := Diff(c, old, 2, 6, new, 2, 6); err != nil {
		t.Fatal(err)
	}
	ops := c.Ops()

	// Reconstruct just the window new[2:6] from old[2:6].
	out := make([]int, 0, 4)
	for _, op := range ops {
		switch op.Tag {
		case tagEqual:
			out = append(out, old[op.OldIndex:op.OldIndex+op.OldLen]...)
		case tagInsert:
			out = append(out, new[op.NewIndex:op.NewIndex+op.NewLen]...)
		}
	}
	if !slicesEqual(out, new[2:6]) {
		t.Fatalf("sub-range reconstruct = %v, want %v", out, new[2:6])
	}
	// All indices must fall inside the requested windows.
	for _, op := range ops {
		os, oe := op.OldRange()
		ns, ne := op.NewRange()
		if os < 2 || oe > 6 || ns < 2 || ne > 6 {
			t.Fatalf("op %v escaped the sub-range windows", op)
		}
	}
}
