package similar

import (
	"reflect"
	"testing"
)

func TestGetCloseMatches(t *testing.T) {
	got := GetCloseMatches("appel", []string{"ape", "apple", "peach", "puppy"}, 3, 0.6)
	want := []string{"apple", "ape"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCloseMatches = %v, want %v", got, want)
	}
}

func TestGetCloseMatchesRespectsN(t *testing.T) {
	got := GetCloseMatches("appel", []string{"ape", "apple", "peach", "puppy"}, 1, 0.6)
	want := []string{"apple"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCloseMatches(n=1) = %v, want %v", got, want)
	}
}

func TestGetCloseMatchesCutoff(t *testing.T) {
	// A high cutoff should exclude everything but a near-exact match.
	got := GetCloseMatches("apple", []string{"ape", "apple", "aple"}, 3, 0.99)
	want := []string{"apple"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCloseMatches(cutoff=0.99) = %v, want %v", got, want)
	}
}

func TestGetCloseMatchesTieBreakLexicographic(t *testing.T) {
	// Two candidates equally similar to the word should come back in
	// lexicographic order.
	got := GetCloseMatches("ab", []string{"cb", "ba", "xb"}, 3, 0.4)
	// "cb", "ba", "xb" each share one char with "ab" -> ratio 0.5; order asc.
	want := []string{"ba", "cb", "xb"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tie-break = %v, want %v", got, want)
	}
}

func TestGetCloseMatchesEmpty(t *testing.T) {
	if got := GetCloseMatches("x", nil, 3, 0.6); len(got) != 0 {
		t.Fatalf("empty possibilities = %v, want none", got)
	}
	if got := GetCloseMatches("apple", []string{"apple"}, 0, 0.6); len(got) != 0 {
		t.Fatalf("n=0 = %v, want none", got)
	}
}

func TestUpperSeqRatio(t *testing.T) {
	if r := upperSeqRatio(nil, nil); r != 1.0 {
		t.Errorf("upperSeqRatio(empty) = %v, want 1.0", r)
	}
	if r := upperSeqRatio([]string{"a", "b"}, []string{"a", "b", "c", "d"}); r != 2.0*2/6 {
		t.Errorf("upperSeqRatio = %v, want %v", r, 2.0*2/6)
	}
}
