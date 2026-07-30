package similar

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

const unknownAlg = Algorithm(99)

func TestCaptureOpsRunsStandardHookStack(t *testing.T) {
	old := []string{"a", "b", "c", "d"}
	new := []string{"a", "x", "y", "d"}

	ops, err := captureOps(context.Background(), Myers, old, new)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	// Replace is only produced by the Replace hook, so seeing one here proves
	// the delete+insert pair was coalesced rather than passed straight through.
	want := []DiffOp{
		{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 1, NewLen: 1},
		{Tag: Replace, OldIndex: 1, NewIndex: 1, OldLen: 2, NewLen: 2},
		{Tag: Equal, OldIndex: 3, NewIndex: 3, OldLen: 1, NewLen: 1},
	}
	if !reflect.DeepEqual(ops, want) {
		t.Fatalf("ops = %v, want %v", ops, want)
	}
}

func TestCaptureOpsIdenticalSequences(t *testing.T) {
	s := []int{1, 2, 3}
	ops, err := captureOps(context.Background(), Myers, s, s)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	want := []DiffOp{{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 3, NewLen: 3}}
	if !reflect.DeepEqual(ops, want) {
		t.Fatalf("ops = %v, want %v", ops, want)
	}
}

func TestCaptureOpsEmptyInputs(t *testing.T) {
	ops, err := captureOps(context.Background(), Myers, []int{}, []int{})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if len(ops) != 0 {
		t.Fatalf("ops = %v, want none", ops)
	}
}

func TestCaptureOpsUnknownAlgorithm(t *testing.T) {
	ops, err := captureOps(context.Background(), unknownAlg, []int{1}, []int{2})
	if err == nil {
		t.Fatal("Capture with unknown algorithm: want error, got nil")
	}
	if ops != nil {
		t.Fatalf("ops = %v, want nil on error", ops)
	}
	if want := "similar: unknown algorithm 99"; err.Error() != want {
		t.Fatalf("err = %q, want %q", err, want)
	}
}

func TestRunDispatchUnknownAlgorithm(t *testing.T) {
	err := run(context.Background(), unknownAlg, NewCapture(), []int{1}, 0, 1, []int{2}, 0, 1)
	if err == nil {
		t.Fatal("Run with unknown algorithm: want error, got nil")
	}
}

func TestRunDispatchDiffsSubRanges(t *testing.T) {
	old := []int{9, 1, 2, 3, 9}
	new := []int{8, 1, 2, 3, 8}

	capture := NewCapture()
	if err := run(context.Background(), Myers, capture, old, 1, 4, new, 1, 4); err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []DiffOp{{Tag: Equal, OldIndex: 1, NewIndex: 1, OldLen: 3, NewLen: 3}}
	if !reflect.DeepEqual(capture.Ops(), want) {
		t.Fatalf("ops = %v, want %v", capture.Ops(), want)
	}
}

// failingHook fails on its first Equal callback, standing in for any hook whose
// callback errors mid-diff.
type failingHook struct {
	NoopHook
	err error
}

func (h *failingHook) Equal(oldIndex, newIndex, length int) error { return h.err }

func TestRunDispatchPropagatesHookError(t *testing.T) {
	boom := errors.New("boom")
	err := run(context.Background(), Myers, &failingHook{err: boom},
		[]int{1, 2}, 0, 2, []int{1, 2}, 0, 2)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

func TestCaptureOpsCancelledContextIsNotAnError(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	old := make([]int, 2000)
	new := make([]int, 2000)
	for i := range old {
		old[i] = i
		new[i] = i * 2
	}

	// An expired deadline yields an approximate but valid script, not an error.
	ops, err := captureOps(ctx, Myers, old, new)
	if err != nil {
		t.Fatalf("Capture with expired deadline: %v", err)
	}
	if len(ops) == 0 {
		t.Fatal("ops = none, want a valid (possibly approximate) script")
	}
}
