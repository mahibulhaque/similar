package engine

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mahibulhaque/similar/internal/algorithms"
	"github.com/mahibulhaque/similar/internal/diff"
)

const unknownAlg = algorithms.Algorithm(99)

func TestCaptureRunsStandardHookStack(t *testing.T) {
	old := []string{"a", "b", "c", "d"}
	new := []string{"a", "x", "y", "d"}

	ops, err := Capture(context.Background(), algorithms.Myers, old, new)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	// Replace is only produced by the Replace hook, so seeing one here proves
	// the delete+insert pair was coalesced rather than passed straight through.
	want := []diff.DiffOp{
		{Tag: diff.Equal, OldIndex: 0, NewIndex: 0, OldLen: 1, NewLen: 1},
		{Tag: diff.Replace, OldIndex: 1, NewIndex: 1, OldLen: 2, NewLen: 2},
		{Tag: diff.Equal, OldIndex: 3, NewIndex: 3, OldLen: 1, NewLen: 1},
	}
	if !reflect.DeepEqual(ops, want) {
		t.Fatalf("ops = %v, want %v", ops, want)
	}
}

func TestCaptureIdenticalSequences(t *testing.T) {
	s := []int{1, 2, 3}
	ops, err := Capture(context.Background(), algorithms.Myers, s, s)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	want := []diff.DiffOp{{Tag: diff.Equal, OldIndex: 0, NewIndex: 0, OldLen: 3, NewLen: 3}}
	if !reflect.DeepEqual(ops, want) {
		t.Fatalf("ops = %v, want %v", ops, want)
	}
}

func TestCaptureEmptyInputs(t *testing.T) {
	ops, err := Capture(context.Background(), algorithms.Myers, []int{}, []int{})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if len(ops) != 0 {
		t.Fatalf("ops = %v, want none", ops)
	}
}

func TestCaptureUnknownAlgorithm(t *testing.T) {
	ops, err := Capture(context.Background(), unknownAlg, []int{1}, []int{2})
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

func TestRunUnknownAlgorithm(t *testing.T) {
	err := Run(context.Background(), unknownAlg, diff.NewCapture(), []int{1}, 0, 1, []int{2}, 0, 1)
	if err == nil {
		t.Fatal("Run with unknown algorithm: want error, got nil")
	}
}

func TestRunDiffsSubRanges(t *testing.T) {
	old := []int{9, 1, 2, 3, 9}
	new := []int{8, 1, 2, 3, 8}

	capture := diff.NewCapture()
	if err := Run(context.Background(), algorithms.Myers, capture, old, 1, 4, new, 1, 4); err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []diff.DiffOp{{Tag: diff.Equal, OldIndex: 1, NewIndex: 1, OldLen: 3, NewLen: 3}}
	if !reflect.DeepEqual(capture.Ops(), want) {
		t.Fatalf("ops = %v, want %v", capture.Ops(), want)
	}
}

// failingHook fails on its first Equal callback, standing in for any hook whose
// callback errors mid-diff.
type failingHook struct {
	diff.NoopHook
	err error
}

func (h *failingHook) Equal(oldIndex, newIndex, length int) error { return h.err }

func TestRunPropagatesHookError(t *testing.T) {
	boom := errors.New("boom")
	err := Run(context.Background(), algorithms.Myers, &failingHook{err: boom},
		[]int{1, 2}, 0, 2, []int{1, 2}, 0, 2)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

func TestCaptureCancelledContextIsNotAnError(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	old := make([]int, 2000)
	new := make([]int, 2000)
	for i := range old {
		old[i] = i
		new[i] = i * 2
	}

	// An expired deadline yields an approximate but valid script, not an error.
	ops, err := Capture(ctx, algorithms.Myers, old, new)
	if err != nil {
		t.Fatalf("Capture with expired deadline: %v", err)
	}
	if len(ops) == 0 {
		t.Fatal("ops = none, want a valid (possibly approximate) script")
	}
}
