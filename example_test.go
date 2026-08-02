package similar_test

import (
	"context"
	"fmt"
	"time"

	"github.com/mahibulhaque/similar"
)

func ExampleDiff() {
	old := []string{"foo", "bar", "baz"}
	new := []string{"foo", "bar", "blah"}

	for _, op := range similar.Diff(old, new) {
		os, oe := op.OldRange()
		ns, ne := op.NewRange()
		fmt.Printf("%s old[%d:%d] new[%d:%d]\n", op.Tag, os, oe, ns, ne)
	}
	// Output:
	// equal old[0:2] new[0:2]
	// replace old[2:3] new[2:3]
}

func ExampleDiff_options() {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// A deadline bounds the work. If it expires the script is still valid, only
	// approximate, so there is nothing to check.
	ops := similar.Diff([]rune("kitten"), []rune("sitting"),
		similar.WithContext(ctx),
		similar.WithAlgorithm(similar.Myers),
	)
	fmt.Println(len(ops) > 0)
	// Output:
	// true
}

// countHook embeds NoopHook and overrides only the callbacks it needs.
type countHook struct {
	similar.NoopHook
	equal, changed int
}

func (h *countHook) Equal(int, int, int) error  { h.equal++; return nil }
func (h *countHook) Delete(int, int, int) error { h.changed++; return nil }
func (h *countHook) Insert(int, int, int) error { h.changed++; return nil }

func ExampleDiffTo() {
	h := &countHook{}
	if err := similar.DiffTo(h, []int{1, 2, 3, 4}, []int{1, 9, 3, 4}); err != nil {
		panic(err)
	}
	fmt.Println("equal runs:", h.equal, "changes:", h.changed)
	// Output:
	// equal runs: 2 changes: 2
}

func ExampleDiffRangeTo() {
	old := []int{9, 1, 2, 3, 9}
	new := []int{8, 1, 2, 3, 8}

	// Diff the middles only. Indices reported to the hook stay absolute, which
	// is what slicing the inputs would cost you.
	capture := similar.NewCapture()
	if err := similar.DiffRangeTo(capture, old, 1, 4, new, 1, 4); err != nil {
		panic(err)
	}
	for _, op := range capture.Ops() {
		os, oe := op.OldRange()
		fmt.Printf("%s old[%d:%d]\n", op.Tag, os, oe)
	}
	// Output:
	// equal old[1:4]
}
