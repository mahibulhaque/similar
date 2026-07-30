package similar_test

import (
	"context"
	"testing"

	"github.com/mahibulhaque/similar"
)

const unknownAlg = similar.Algorithm(99)

func TestDiffDeadlineRejectsUnknownAlgorithm(t *testing.T) {
	ops, err := similar.DiffDeadline(context.Background(), unknownAlg, []int{1}, []int{2})
	if err == nil {
		t.Fatal("DiffDeadline with unknown algorithm: want error, got nil")
	}
	if ops != nil {
		t.Fatalf("ops = %v, want nil on error", ops)
	}
}

func TestDiffHookDeadlineRejectsUnknownAlgorithm(t *testing.T) {
	err := similar.DiffHookDeadline(context.Background(), unknownAlg, similar.NewCapture(), []int{1}, []int{2})
	if err == nil {
		t.Fatal("DiffHookDeadline with unknown algorithm: want error, got nil")
	}
}

func TestDiffRangeHookDeadlineRejectsUnknownAlgorithm(t *testing.T) {
	err := similar.DiffRangeHookDeadline(context.Background(), unknownAlg, similar.NewCapture(),
		[]int{1}, 0, 1, []int{2}, 0, 1)
	if err == nil {
		t.Fatal("DiffRangeHookDeadline with unknown algorithm: want error, got nil")
	}
}

// CaptureDiff cannot report the failure, so it panics rather than returning a
// nil script that reads as "no differences".
func TestCaptureDiffPanicsOnUnknownAlgorithm(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("CaptureDiff with unknown algorithm: want panic, got none")
		}
		if want := "similar: unknown algorithm 99"; r != want {
			t.Fatalf("panic = %v, want %q", r, want)
		}
	}()
	similar.CaptureDiff(unknownAlg, []int{1}, []int{2})
}

func TestCaptureDiffMyersMatchesDiff(t *testing.T) {
	old := []string{"a", "b", "c"}
	new := []string{"a", "x", "c"}
	got := similar.CaptureDiff(similar.Myers, old, new)
	want := similar.Diff(old, new)
	if len(got) != len(want) {
		t.Fatalf("CaptureDiff = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("CaptureDiff = %v, want %v", got, want)
		}
	}
}

// The text layer honors WithAlgorithm instead of recording it and diffing with
// Myers regardless.
func TestTextWithAlgorithmPanicsOnUnknown(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("WithAlgorithm(99): want panic, got none")
		}
	}()
	similar.WithAlgorithm(unknownAlg)
}
