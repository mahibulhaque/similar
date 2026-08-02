package algorithms

import (
	"testing"
)

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
