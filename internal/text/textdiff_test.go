package text

import (
	"reflect"
	"slices"
	"testing"

	"github.com/mahibulhaque/similar/internal/diff"
)

func TestDiffLinesOps(t *testing.T) {
	d := DiffLines("a\nb\nc", "a\nb\nC")
	want := []diff.DiffOp{
		{Tag: diff.Equal, OldIndex: 0, NewIndex: 0, OldLen: 2, NewLen: 2},
		{Tag: diff.Replace, OldIndex: 2, NewIndex: 2, OldLen: 1, NewLen: 1},
	}
	if !reflect.DeepEqual(d.Ops(), want) {
		t.Fatalf("ops = %v, want %v", d.Ops(), want)
	}
	if !d.NewlineTerminated() {
		t.Errorf("line diff should be newline-terminated by default")
	}
}

func TestDiffWordsOps(t *testing.T) {
	d := DiffWords("foo bar baz", "foo BAR baz")
	want := []diff.DiffOp{
		{Tag: diff.Equal, OldIndex: 0, NewIndex: 0, OldLen: 2, NewLen: 2},
		{Tag: diff.Replace, OldIndex: 2, NewIndex: 2, OldLen: 1, NewLen: 1},
		{Tag: diff.Equal, OldIndex: 3, NewIndex: 3, OldLen: 2, NewLen: 2},
	}
	if !reflect.DeepEqual(d.Ops(), want) {
		t.Fatalf("ops = %v, want %v", d.Ops(), want)
	}
	if d.NewlineTerminated() {
		t.Errorf("word diff should not be newline-terminated by default")
	}
}

func TestDiffCharsOps(t *testing.T) {
	d := DiffChars("abcdef", "abcDDf")
	want := []diff.DiffOp{
		{Tag: diff.Equal, OldIndex: 0, NewIndex: 0, OldLen: 3, NewLen: 3},
		{Tag: diff.Replace, OldIndex: 3, NewIndex: 3, OldLen: 2, NewLen: 2},
		{Tag: diff.Equal, OldIndex: 5, NewIndex: 5, OldLen: 1, NewLen: 1},
	}
	if !reflect.DeepEqual(d.Ops(), want) {
		t.Fatalf("ops = %v, want %v", d.Ops(), want)
	}
}

func TestDiffSlicesOps(t *testing.T) {
	old := []string{"foo", "bar", "baz"}
	new := []string{"foo", "BAR", "baz"}
	d := DiffSlices(old, new)
	want := []diff.DiffOp{
		{Tag: diff.Equal, OldIndex: 0, NewIndex: 0, OldLen: 1, NewLen: 1},
		{Tag: diff.Replace, OldIndex: 1, NewIndex: 1, OldLen: 1, NewLen: 1},
		{Tag: diff.Equal, OldIndex: 2, NewIndex: 2, OldLen: 1, NewLen: 1},
	}
	if !reflect.DeepEqual(d.Ops(), want) {
		t.Fatalf("ops = %v, want %v", d.Ops(), want)
	}
	// Mutating the caller's slice must not affect the diff.
	old[1] = "mutated"
	if tok, _ := d.OldToken(1); tok != "bar" {
		t.Errorf("DiffSlices did not copy input: OldToken(1) = %q, want %q", tok, "bar")
	}
}

func TestRatio(t *testing.T) {
	if got := DiffChars("abcd", "bcde").Ratio(); got != 0.75 {
		t.Errorf("Ratio = %v, want 0.75", got)
	}
	if got := DiffChars("", "").Ratio(); got != 1.0 {
		t.Errorf("empty Ratio = %v, want 1.0", got)
	}
}

func TestTokenAccessors(t *testing.T) {
	d := DiffLines("a\nb\nc", "a\nb\nC")
	if d.OldLen() != 3 || d.NewLen() != 3 {
		t.Fatalf("lens = (%d,%d), want (3,3)", d.OldLen(), d.NewLen())
	}
	if tok, ok := d.OldToken(0); !ok || tok != "a\n" {
		t.Errorf("OldToken(0) = (%q,%v), want (\"a\\n\",true)", tok, ok)
	}
	if _, ok := d.OldToken(3); ok {
		t.Errorf("OldToken(3) should be out of range")
	}
	got := slices.Collect(d.OldTokens())
	if want := []string{"a\n", "b\n", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("OldTokens = %q, want %q", got, want)
	}
}

func TestGroupedOpsFromTextDiff(t *testing.T) {
	// A diff with a long equal run should split into two groups.
	oldLines := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\nX\n"
	newLines := "A\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\nX\n"
	d := DiffLines(oldLines, newLines)
	groups := d.GroupedOps(1)
	if len(groups) == 0 {
		t.Fatalf("expected at least one group, got none (ops=%v)", d.Ops())
	}
}
