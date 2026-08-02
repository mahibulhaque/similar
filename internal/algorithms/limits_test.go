package algorithms

import (
	"context"
	"testing"
	"time"
)

// TestDefaultLimits pins the production gate values. Every other test in this
// file moves a gate deliberately; this is the one that would notice a default
// changing by accident.
func TestDefaultLimits(t *testing.T) {
	lim := fromContext(context.Background())

	cases := []struct {
		name string
		got  int
		want int
	}{
		{"smallSideExactMax", lim.smallSideExactMax, 64},
		{"smallSideExactMinLarge", lim.smallSideExactMinLarge, 512},
		{"smallSideExactMaxWork", lim.smallSideExactMaxWork, 64_000_000},
		{"disjointFastPathMinLen", lim.disjointFastPathMinLen, 512},
		{"disjointFastPathMinWork", lim.disjointFastPathMinWork, 128 * 1024},
		{"frontAnchorMaxSkip", lim.frontAnchorMaxSkip, 4},
		{"frontAnchorMinCommon", lim.frontAnchorMinCommon, 96},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
		}
	}

	if lim.exceeded() {
		t.Error("a background context must not be an exceeded deadline")
	}
}

// TestFromContextNilContext covers the branch fromContext guards with
// "if ctx != nil". A nil context is a supported input — noDeadline is built
// from one — so the path is exercised through a typed variable rather than a
// literal, which staticcheck rejects on sight.
func TestFromContextNilContext(t *testing.T) {
	var none context.Context

	lim := fromContext(none)
	if lim.exceeded() {
		t.Fatal("a nil context must not read as exceeded")
	}
	if !lim.time.IsZero() {
		t.Fatalf("deadline = %v, want zero", lim.time)
	}
	if lim.smallSideExactMax != defaultSmallSideExactMax {
		t.Fatalf("gates not defaulted: smallSideExactMax = %d", lim.smallSideExactMax)
	}
}

func TestFromContextReadsDeadlineOnce(t *testing.T) {
	want := time.Now().Add(time.Hour)
	ctx, cancel := context.WithDeadline(context.Background(), want)
	defer cancel()

	lim := fromContext(ctx)
	if !lim.time.Equal(want) {
		t.Fatalf("deadline = %v, want %v", lim.time, want)
	}
	if lim.exceeded() {
		t.Fatal("a deadline an hour out must not read as exceeded")
	}
}

func TestFromContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	lim := fromContext(ctx)
	if lim.exceeded() {
		t.Fatal("live context read as exceeded")
	}
	cancel()
	if !lim.exceeded() {
		t.Fatal("cancelled context did not read as exceeded")
	}
}

// TestSmallSideCapClamps guards the reason smallSideCap exists: the exact
// fallback's dp table stores LCS lengths in a uint8, so a small side longer
// than 255 would wrap. Lowering the gate is honoured; raising it is capped.
func TestSmallSideCapClamps(t *testing.T) {
	lim := fromContext(context.Background())

	lim.smallSideExactMax = 8
	if got := lim.smallSideCap(); got != 8 {
		t.Fatalf("lowered cap = %d, want 8", got)
	}

	lim.smallSideExactMax = 100_000
	if got := lim.smallSideCap(); got != smallSideExactHardMax {
		t.Fatalf("raised cap = %d, want %d", got, smallSideExactHardMax)
	}
}

// The three tests below are what folding the gates into limits bought. Each
// drives a heuristic across its own threshold on inputs of five to ten items,
// where before it would have taken several hundred to cross a hard-coded 512.

func TestSmallSideExactAtLoweredGates(t *testing.T) {
	lim := fromContext(context.Background())
	lim.smallSideExactMax = 2
	lim.smallSideExactMinLarge = 5
	lim.smallSideExactMaxWork = 1000

	old := []int{2, 4}
	new := []int{1, 2, 3, 4, 5}

	t.Run("fires and stays exact", func(t *testing.T) {
		c := newCapture()
		used, err := maybeEmitSmallSideExact(c, old, 0, len(old), new, 0, len(new), lim)
		if err != nil {
			t.Fatal(err)
		}
		if !used {
			t.Fatal("expected small-side-exact to be used")
		}
		assertInvariants(t, old, new, c.Ops())
	})

	t.Run("declines one under the small-side gate", func(t *testing.T) {
		tight := lim
		tight.smallSideExactMax = len(old) - 1

		c := newCapture()
		used, err := maybeEmitSmallSideExact(c, old, 0, len(old), new, 0, len(new), tight)
		if err != nil {
			t.Fatal(err)
		}
		if used {
			t.Fatal("expected small-side-exact to decline")
		}
	})

	t.Run("declines one over the large-side gate", func(t *testing.T) {
		tight := lim
		tight.smallSideExactMinLarge = len(new) + 1

		c := newCapture()
		used, err := maybeEmitSmallSideExact(c, old, 0, len(old), new, 0, len(new), tight)
		if err != nil {
			t.Fatal(err)
		}
		if used {
			t.Fatal("expected small-side-exact to decline")
		}
	})

	t.Run("declines when the work cap is under the product", func(t *testing.T) {
		tight := lim
		tight.smallSideExactMaxWork = len(old)*len(new) - 1

		c := newCapture()
		used, err := maybeEmitSmallSideExact(c, old, 0, len(old), new, 0, len(new), tight)
		if err != nil {
			t.Fatal(err)
		}
		if used {
			t.Fatal("expected small-side-exact to decline")
		}
	})
}

func TestDisjointFastPathAtLoweredGates(t *testing.T) {
	lim := fromContext(context.Background())
	lim.disjointFastPathMinLen = 3
	lim.disjointFastPathMinWork = 9

	old := []int{1, 2, 3}
	new := []int{4, 5, 6}

	c := newCapture()
	used, err := maybeEmitDisjointFastPath(c, old, 0, len(old), new, 0, len(new), lim)
	if err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Fatal("expected the disjoint fast path to be used")
	}
	assertInvariants(t, old, new, c.Ops())

	// One over the work gate and the same input is refused.
	tight := lim
	tight.disjointFastPathMinWork = len(old)*len(new) + 1

	c2 := newCapture()
	used2, err := maybeEmitDisjointFastPath(c2, old, 0, len(old), new, 0, len(new), tight)
	if err != nil {
		t.Fatal(err)
	}
	if used2 {
		t.Fatal("expected the disjoint fast path to decline above its work gate")
	}
}

func TestFrontAnchorAtLoweredGates(t *testing.T) {
	lim := fromContext(context.Background())
	lim.frontAnchorMinCommon = 3
	lim.frontAnchorMaxSkip = 2

	// Ten against four, so the larger side is at least twice the smaller. The
	// anchor should peel the single leading item and match the rest.
	old := []int{99, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	new := []int{1, 2, 3, 4}

	c := newCapture()
	oldNext, newNext, err := tryEmitFrontAnchor(c, old, 0, len(old), new, 0, len(new), lim)
	if err != nil {
		t.Fatal(err)
	}
	if oldNext == 0 && newNext == 0 {
		t.Fatal("expected the anchor scan to advance the ranges")
	}

	want := []capturedOp{
		{Tag: tagDelete, OldIndex: 0, NewIndex: 0, OldLen: 1},
		{Tag: tagEqual, OldIndex: 1, NewIndex: 0, OldLen: 4, NewLen: 4},
	}
	got := c.Ops()
	if len(got) != len(want) {
		t.Fatalf("ops = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("op %d = %v, want %v", i, got[i], want[i])
		}
	}
	if oldNext != 5 || newNext != 4 {
		t.Fatalf("resumed at (%d,%d), want (5,4)", oldNext, newNext)
	}

	// Raising the common-run gate above what the input offers declines it.
	tight := lim
	tight.frontAnchorMinCommon = len(new) + 1

	c2 := newCapture()
	oldNext2, newNext2, err := tryEmitFrontAnchor(c2, old, 0, len(old), new, 0, len(new), tight)
	if err != nil {
		t.Fatal(err)
	}
	if oldNext2 != 0 || newNext2 != 0 || len(c2.Ops()) != 0 {
		t.Fatalf("expected no anchor: (%d,%d) ops=%v", oldNext2, newNext2, c2.Ops())
	}
}
