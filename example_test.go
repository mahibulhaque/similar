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

func ExampleCaptureDiff() {
	old := []int{1, 2, 3, 4, 5}
	new := []int{1, 2, 3, 4, 7}

	ops := similar.CaptureDiff(similar.Myers, old, new)
	fmt.Println(len(ops), "ops")
	fmt.Println(ops[len(ops)-1].Tag)
	// Output:
	// 2 ops
	// replace
}

func ExampleDiffDeadline() {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	ops, err := similar.DiffDeadline(ctx, similar.Myers, []rune("kitten"), []rune("sitting"))
	if err != nil {
		panic(err)
	}
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

func ExampleDiffHookDeadline() {
	h := &countHook{}
	err := similar.DiffHookDeadline(context.Background(), similar.Myers, h,
		[]int{1, 2, 3, 4}, []int{1, 9, 3, 4})
	if err != nil {
		panic(err)
	}
	fmt.Println("equal runs:", h.equal, "changes:", h.changed)
	// Output:
	// equal runs: 2 changes: 2
}
