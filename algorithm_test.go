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

// LCS reaches the same place by another route: both algorithms produce a minimal
// script, and the hook stack above them compacts both the same way, so on these
// shapes the two agree operation for operation.
func TestWithAlgorithmLCSMatchesMyers(t *testing.T) {
	cases := []struct {
		name     string
		old, new []string
	}{
		{"replaced middle", []string{"a", "b", "c"}, []string{"a", "x", "c"}},
		{"pure insert", nil, []string{"a", "b"}},
		{"pure delete", []string{"a", "b"}, nil},
		{"both empty", nil, nil},
		{"identical", []string{"a", "b"}, []string{"a", "b"}},
		{"disjoint", []string{"a", "b"}, []string{"c", "d"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := similar.Diff(tc.old, tc.new, similar.WithAlgorithm(similar.LCS))
			want := similar.Diff(tc.old, tc.new, similar.WithAlgorithm(similar.Myers))
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("LCS = %v, Myers = %v", got, want)
			}
			if out := reconstruct(tc.old, tc.new, got); !eq(out, tc.new) {
				t.Fatalf("LCS script rebuilt %v, want %v", out, tc.new)
			}
		})
	}
}

// The text layer shares the option, so it honours the choice rather than
// recording it and diffing with Myers regardless.
func TestTextLayerHonorsWithAlgorithm(t *testing.T) {
	for _, alg := range []similar.Algorithm{similar.Myers, similar.LCS} {
		t.Run(alg.String(), func(t *testing.T) {
			d := similar.DiffLines("a\nb\n", "a\nc\n", similar.WithAlgorithm(alg))
			if got := d.Algorithm(); got != alg {
				t.Fatalf("Algorithm = %v, want %v", got, alg)
			}
			if got, want := d.Ratio(), similar.DiffLines("a\nb\n", "a\nc\n").Ratio(); got != want {
				t.Fatalf("Ratio = %v, want %v", got, want)
			}
		})
	}
}

func TestAlgorithmString(t *testing.T) {
	if got, want := similar.Myers.String(), "myers"; got != want {
		t.Errorf("Myers.String() = %q, want %q", got, want)
	}
	if got, want := similar.LCS.String(), "lcs"; got != want {
		t.Errorf("LCS.String() = %q, want %q", got, want)
	}
	if got, want := unknownAlg.String(), "Algorithm(99)"; got != want {
		t.Errorf("unknown.String() = %q, want %q", got, want)
	}
}
