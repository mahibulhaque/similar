package algorithms

import (
	"reflect"
	"testing"
)

// patienceOp is a recorded call to the diffHook interface. oldLen/newLen
// hold whichever length argument that op carries: for Equal both are the
// same shared length, for Delete only oldLen is meaningful, for Insert only
// newLen is.
type patienceOp struct {
	kind             string // "equal", "delete", or "insert"
	oldIndex, oldLen int
	newIndex, newLen int
}

// patienceCapture is a minimal diffHook that records every call it receives,
// including how many times Finish was called.
type patienceCapture struct {
	ops         []patienceOp
	finishCalls int
}

func (c *patienceCapture) Equal(oldIndex, newIndex, length int) error {
	c.ops = append(c.ops, patienceOp{"equal", oldIndex, length, newIndex, length})
	return nil
}

func (c *patienceCapture) Delete(oldIndex, oldLen, newIndex int) error {
	c.ops = append(c.ops, patienceOp{"delete", oldIndex, oldLen, newIndex, 0})
	return nil
}

func (c *patienceCapture) Insert(oldIndex, newIndex, newLen int) error {
	c.ops = append(c.ops, patienceOp{"insert", oldIndex, 0, newIndex, newLen})
	return nil
}

func (c *patienceCapture) Finish() error {
	c.finishCalls++
	return nil
}

// reconstructNew replays a script of ops against old and new, using only the
// indices/lengths in the script (never trusting anything else about how the
// script was produced), and returns the sequence it reconstructs. A correct
// script reconstructs new exactly, regardless of exactly how the diff chose
// to get there — which is what lets these tests avoid pinning down an exact
// op sequence that depends on internal Myers heuristics.
func reconstructNew[T comparable](old, new []T, ops []patienceOp) []T {
	got := make([]T, 0, len(new))
	for _, op := range ops {
		switch op.kind {
		case "equal":
			got = append(got, old[op.oldIndex:op.oldIndex+op.oldLen]...)
		case "insert":
			got = append(got, new[op.newIndex:op.newIndex+op.newLen]...)
		case "delete":
			// contributes nothing to new
		}
	}
	return got
}

func runPatience[T comparable](t *testing.T, old, new []T) *patienceCapture {
	t.Helper()
	c := &patienceCapture{}
	if err := DiffPatience[T](c, old, 0, len(old), new, 0, len(new)); err != nil {
		t.Fatalf("DiffPatience returned error: %v", err)
	}
	return c
}

func TestDiffPatience_ReconstructsNew(t *testing.T) {
	tests := []struct {
		name     string
		old, new []int
	}{
		{
			name: "typical mixed changes",
			old:  []int{11, 1, 2, 2, 3, 4, 4, 4, 5, 47, 19},
			new:  []int{10, 1, 2, 2, 8, 9, 4, 4, 7, 47, 18},
		},
		{
			// Every value in both ranges is repeated, so diffutil.Unique
			// finds no anchors at all and patience must fall back entirely
			// to a plain nested Myers diff over the whole range.
			name: "no unique anchors",
			old:  []int{1, 1, 0, 0},
			new:  []int{0, 1, 1, 0},
		},
		{
			name: "old longer than new",
			old:  []int{1, 2, 3, 4},
			new:  []int{1, 2, 3},
		},
		{
			name: "new longer than old",
			old:  []int{1, 2, 3},
			new:  []int{1, 2, 3, 4},
		},
		{
			name: "identical slices",
			old:  []int{1, 2, 3},
			new:  []int{1, 2, 3},
		},
		{
			name: "both empty",
			old:  []int{},
			new:  []int{},
		},
		{
			name: "old empty",
			old:  []int{},
			new:  []int{1, 2, 3},
		},
		{
			name: "new empty",
			old:  []int{1, 2, 3},
			new:  []int{},
		},
		{
			name: "completely disjoint",
			old:  []int{1, 2, 3},
			new:  []int{4, 5, 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := runPatience(t, tt.old, tt.new)

			got := reconstructNew(tt.old, tt.new, c.ops)
			if !reflect.DeepEqual(got, tt.new) {
				t.Errorf("reconstructed %v, want %v (ops: %+v)", got, tt.new, c.ops)
			}

			if c.finishCalls != 1 {
				t.Errorf("Finish called %d times, want exactly 1", c.finishCalls)
			}
		})
	}
}

func TestDiffPatience_FinishCalledOnceAcrossManyAnchors(t *testing.T) {
	// Enough distinct values that every element is a unique anchor, so this
	// exercises the case where Equal fires many times before finishOuter.
	old := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	new := []int{1, 2, 30, 4, 5, 60, 7, 8, 9, 100}

	c := runPatience(t, old, new)

	got := reconstructNew(old, new, c.ops)
	if !reflect.DeepEqual(got, new) {
		t.Errorf("reconstructed %v, want %v (ops: %+v)", got, new, c.ops)
	}
	if c.finishCalls != 1 {
		t.Errorf("Finish called %d times, want exactly 1", c.finishCalls)
	}
}

func TestDiffPatience_Strings(t *testing.T) {
	// diffutil.Unique and patienceHook are generic; exercise a non-int T to
	// make sure nothing here accidentally assumes int.
	old := []string{"a", "b", "c", "d"}
	new := []string{"a", "x", "c", "d"}

	c := &patienceCapture{}
	if err := DiffPatience[string](c, old, 0, len(old), new, 0, len(new)); err != nil {
		t.Fatalf("DiffPatience returned error: %v", err)
	}

	got := reconstructNew(old, new, c.ops)
	if !reflect.DeepEqual(got, new) {
		t.Errorf("reconstructed %v, want %v (ops: %+v)", got, new, c.ops)
	}
}
