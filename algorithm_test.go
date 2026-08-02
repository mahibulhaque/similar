package similar_test

import (
	"reflect"
	"testing"

	"github.com/mahibulhaque/similar"
)

const unknownAlg = similar.Algorithm(99)

// WithAlgorithm is the only way an Algorithm enters the package, so it is the
// only place a bad one can be caught — and it catches it where the caller can
// see which argument was wrong, rather than at some later diff that has no
// error to return.
func TestWithAlgorithmIsTheOnlyGate(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("WithAlgorithm(99): want panic, got none")
		}
		if want := "similar: unknown algorithm 99"; r != want {
			t.Fatalf("panic = %v, want %q", r, want)
		}
	}()
	similar.WithAlgorithm(unknownAlg)
}

func TestWithAlgorithmMyersMatchesTheDefault(t *testing.T) {
	old := []string{"a", "b", "c"}
	new := []string{"a", "x", "c"}

	got := similar.Diff(old, new, similar.WithAlgorithm(similar.Myers))
	want := similar.Diff(old, new)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("explicit Myers = %v, want %v", got, want)
	}
}

// The text layer shares the option, so it honours the choice rather than
// recording it and diffing with Myers regardless.
func TestTextLayerHonorsWithAlgorithm(t *testing.T) {
	d := similar.DiffLines("a\nb\n", "a\nc\n", similar.WithAlgorithm(similar.Myers))
	if got := d.Algorithm(); got != similar.Myers {
		t.Fatalf("Algorithm = %v, want %v", got, similar.Myers)
	}
}

func TestAlgorithmString(t *testing.T) {
	if got, want := similar.Myers.String(), "myers"; got != want {
		t.Errorf("Myers.String() = %q, want %q", got, want)
	}
	if got, want := unknownAlg.String(), "Algorithm(99)"; got != want {
		t.Errorf("unknown.String() = %q, want %q", got, want)
	}
}
