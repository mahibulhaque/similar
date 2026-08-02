package algorithms

import (
	"testing"
	"time"
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
	old, new := disjointRanges(disjointFastPathMinLen)

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
	n := disjointFastPathMinLen

	cases := []struct {
		name  string
		build func() (old, new []int, dl deadline)
	}{
		{"below the length threshold", func() ([]int, []int, deadline) {
			old, new := disjointRanges(n - 1)
			return old, new, noDeadline
		}},
		{"deadline already exceeded", func() ([]int, []int, deadline) {
			old, new := disjointRanges(n)
			return old, new, deadline{time: time.Now().Add(-time.Second)}
		}},
		{"ranges share an item", func() ([]int, []int, deadline) {
			old, new := disjointRanges(n)
			new[n/2] = old[n/2]
			return old, new, noDeadline
		}},
		{"first items match", func() ([]int, []int, deadline) {
			old, new := disjointRanges(n)
			new[0] = old[0]
			return old, new, noDeadline
		}},
		{"last items match", func() ([]int, []int, deadline) {
			old, new := disjointRanges(n)
			new[n-1] = old[n-1]
			return old, new, noDeadline
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			old, new, dl := tc.build()
			c := newCapture()
			used, err := maybeEmitDisjointFastPath(c, old, 0, len(old), new, 0, len(new), dl)
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
	expired := deadline{time: time.Now().Add(-time.Second)}

	// ok is false when the scan bails before the answer is known.
	if common, ok := hasCommonItem(old, 0, len(old), new, 0, len(new), expired); ok || common {
		t.Fatalf("expired deadline: common = %v, ok = %v, want false, false", common, ok)
	}
}
