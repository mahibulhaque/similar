package algorithms

import (
	"testing"
)

// disjointRanges builds two ranges of length n sharing no element, with
// differing endpoints, so none of the fast path's cheap rejections fire.
func disjointRanges(n int) (old, new []int) {
	old = make([]int, n)
	new = make([]int, n)
	for i := range old {
		old[i] = i
		new[i] = i + n
	}
	return old, new
}

// TestDisjointFastPathEmitsWholeScript pins that the fast path fires on inputs
// at its threshold and emits a complete delete+insert script. The output is the
// same one full search would produce, so no downstream test can tell the path
// apart by its result; this is the only place that observes it directly.
func TestDisjointFastPathEmitsWholeScript(t *testing.T) {
	old, new := disjointRanges(defaultDisjointFastPathMinLen)

	c := newCapture()
	used, err := maybeEmitDisjointFastPath(c, old, 0, len(old), new, 0, len(new), noDeadline)
	if err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Fatal("expected the disjoint fast path to be used")
	}

	want := []capturedOp{
		{Tag: tagDelete, OldIndex: 0, NewIndex: 0, OldLen: len(old)},
		{Tag: tagInsert, OldIndex: 0, NewIndex: 0, NewLen: len(new)},
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
}

// TestDisjointFastPathDeclines breaks exactly one precondition per case,
// starting from ranges the fast path would otherwise accept.
func TestDisjointFastPathDeclines(t *testing.T) {
	n := defaultDisjointFastPathMinLen

	cases := []struct {
		name  string
		build func() (old, new []int, lim limits)
	}{
		{"below the length threshold", func() ([]int, []int, limits) {
			old, new := disjointRanges(n - 1)
			return old, new, noDeadline
		}},
		{"deadline already exceeded", func() ([]int, []int, limits) {
			old, new := disjointRanges(n)
			return old, new, expired()
		}},
		{"ranges share an item", func() ([]int, []int, limits) {
			old, new := disjointRanges(n)
			new[n/2] = old[n/2]
			return old, new, noDeadline
		}},
		{"first items match", func() ([]int, []int, limits) {
			old, new := disjointRanges(n)
			new[0] = old[0]
			return old, new, noDeadline
		}},
		{"last items match", func() ([]int, []int, limits) {
			old, new := disjointRanges(n)
			new[n-1] = old[n-1]
			return old, new, noDeadline
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			old, new, lim := tc.build()
			c := newCapture()
			used, err := maybeEmitDisjointFastPath(c, old, 0, len(old), new, 0, len(new), lim)
			if err != nil {
				t.Fatal(err)
			}
			if used {
				t.Fatal("expected the disjoint fast path to decline")
			}
			if ops := c.Ops(); len(ops) != 0 {
				t.Fatalf("declined but emitted %v", ops)
			}
		})
	}
}

func TestHasCommonItem(t *testing.T) {
	old, new := disjointRanges(8)

	if common, ok := hasCommonItem(old, 0, len(old), new, 0, len(new), noDeadline); !ok || common {
		t.Fatalf("disjoint ranges: common = %v, ok = %v, want false, true", common, ok)
	}

	new[3] = old[5]
	if common, ok := hasCommonItem(old, 0, len(old), new, 0, len(new), noDeadline); !ok || !common {
		t.Fatalf("overlapping ranges: common = %v, ok = %v, want true, true", common, ok)
	}

	// A shared item outside the compared sub-ranges does not count.
	if common, ok := hasCommonItem(old, 0, 5, new, 3, 4, noDeadline); !ok || common {
		t.Fatalf("sub-range excluding the shared item: common = %v, ok = %v, want false, true", common, ok)
	}
}

func TestHasCommonItemDeadline(t *testing.T) {
	old, new := disjointRanges(8)

	// ok is false when the scan bails before the answer is known.
	if common, ok := hasCommonItem(old, 0, len(old), new, 0, len(new), expired()); ok || common {
		t.Fatalf("expired deadline: common = %v, ok = %v, want false, false", common, ok)
	}
}

// TestSmallSideExactVariants ports the Rust `test_small_side_exact_variants`
// fixture: all three shapes the exact fallback handles, judged against the ops
// it emits directly rather than through a full diff, since no downstream result
// can tell the fallback apart from full search.
func TestSmallSideExactVariants(t *testing.T) {
	t.Run("tiny old vs large new", func(t *testing.T) {
		old := []int{1}
		new := make([]int, 1000)
		for i := range new {
			new[i] = i + 10
		}
		c := newCapture()
		used, err := maybeEmitSmallSideExact(c, old, 0, len(old), new, 0, len(new), noDeadline)
		if err != nil {
			t.Fatal(err)
		}
		if !used {
			t.Fatal("expected small-side-exact to be used")
		}
		want := []capturedOp{
			{Tag: tagDelete, OldIndex: 0, NewIndex: 0, OldLen: 1},
			{Tag: tagInsert, OldIndex: 1, NewIndex: 0, NewLen: 1000},
		}
		if len(c.Ops()) != len(want) {
			t.Fatalf("ops = %v, want %v", c.Ops(), want)
		}
		for i, op := range c.Ops() {
			if op != want[i] {
				t.Fatalf("op %d = %v, want %v", i, op, want[i])
			}
		}
	})

	t.Run("sparse overlap far into larger side", func(t *testing.T) {
		old := make([]int, 8)
		for i := range old {
			old[i] = i
		}
		new := make([]int, 1000)
		for i := range new {
			new[i] = 1000 + i
		}
		new[500] = 0
		c := newCapture()
		used, err := maybeEmitSmallSideExact(c, old, 0, len(old), new, 0, len(new), noDeadline)
		if err != nil {
			t.Fatal(err)
		}
		if !used {
			t.Fatal("expected small-side-exact to be used")
		}
		found := false
		for _, op := range c.Ops() {
			if op.Tag == tagEqual && op.OldIndex == 0 && op.NewIndex == 500 && op.OldLen == 1 {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected an Equal at old=0 new=500 len=1; ops=%v", c.Ops())
		}
	})

	t.Run("tiny new vs large old", func(t *testing.T) {
		old := make([]int, 1000)
		for i := range old {
			old[i] = i
		}
		new := []int{500}
		c := newCapture()
		used, err := maybeEmitSmallSideExact(c, old, 0, len(old), new, 0, len(new), noDeadline)
		if err != nil {
			t.Fatal(err)
		}
		if !used {
			t.Fatal("expected small-side-exact to be used")
		}
		totalDeleted, insertCount := 0, 0
		sawExpectedEqual := false
		for _, op := range c.Ops() {
			switch op.Tag {
			case tagDelete:
				totalDeleted += op.OldLen
			case tagInsert:
				insertCount++
			case tagEqual:
				if op.OldIndex == 500 && op.NewIndex == 0 && op.OldLen == 1 {
					sawExpectedEqual = true
				}
			}
		}
		if insertCount != 0 {
			t.Fatalf("insertCount = %d, want 0", insertCount)
		}
		if totalDeleted != 999 {
			t.Fatalf("totalDeleted = %d, want 999", totalDeleted)
		}
		if !sawExpectedEqual {
			t.Fatal("expected Equal at old=500 new=0 len=1")
		}
	})
}
