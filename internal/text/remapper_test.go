package text

import (
	"reflect"
	"slices"
	"testing"

	"github.com/mahibulhaque/similar/internal/diff"
)

func TestRemappedChangesWordDiff(t *testing.T) {
	d := DiffWords("foo bar baz", "foo bor baz")

	got := slices.Collect(d.AllRemappedChanges())
	want := []RemappedChange{
		{diff.ChangeEqual, "foo "},
		{diff.ChangeDelete, "bar"},
		{diff.ChangeInsert, "bor"},
		{diff.ChangeEqual, " baz"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("remapped = %+v, want %+v", got, want)
	}
}

func TestRemappedChangesCharDiffConnectsRuns(t *testing.T) {
	d := DiffChars("foobarbaz", "fooBARbaz")

	got := slices.Collect(d.AllRemappedChanges())
	want := []RemappedChange{
		{diff.ChangeEqual, "foo"},
		{diff.ChangeDelete, "bar"},
		{diff.ChangeInsert, "BAR"},
		{diff.ChangeEqual, "baz"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("remapped = %+v, want %+v", got, want)
	}
}

// RemappedChanges is the per-op form of AllRemappedChanges; a Replace expands to
// a delete run followed by an insert run.
func TestRemappedChangesPerOp(t *testing.T) {
	d := DiffWords("foo bar baz", "foo bor baz")

	var got []RemappedChange
	for _, op := range d.Ops() {
		got = append(got, d.RemappedChanges(op)...)
	}
	if want := slices.Collect(d.AllRemappedChanges()); !reflect.DeepEqual(got, want) {
		t.Fatalf("per-op = %+v, want %+v", got, want)
	}
}

func TestAllRemappedChangesStopsEarly(t *testing.T) {
	d := DiffChars("foobarbaz", "fooBARbaz")

	count := 0
	for range d.AllRemappedChanges() {
		count++
		break
	}
	if count != 1 {
		t.Fatalf("yielded %d changes after break, want 1", count)
	}
}

func TestSliceBounds(t *testing.T) {
	d := DiffWords("foo bar baz", "foo bor baz")

	if s, ok := d.SliceOld(0, 2); !ok || s != "foo " {
		t.Errorf("SliceOld(0,2) = (%q,%v), want (\"foo \",true)", s, ok)
	}
	if s, ok := d.SliceNew(2, 3); !ok || s != "bor" {
		t.Errorf("SliceNew(2,3) = (%q,%v), want (\"bor\",true)", s, ok)
	}
	if s, ok := d.SliceOld(1, 1); !ok || s != "" {
		t.Errorf("SliceOld(1,1) = (%q,%v), want (\"\",true)", s, ok)
	}
	if _, ok := d.SliceOld(0, 100); ok {
		t.Error("SliceOld past the last token should report !ok")
	}
	if _, ok := d.SliceOld(-1, 1); ok {
		t.Error("SliceOld with a negative start should report !ok")
	}
	if _, ok := d.SliceOld(2, 1); ok {
		t.Error("SliceOld with end < start should report !ok")
	}
}

// A diff built from tokens the caller supplied remaps against those tokens, so
// no original string has to be threaded back in.
func TestSlicesFromCallerTokens(t *testing.T) {
	d := DiffSlices(tokenizeWords("foo bar baz"), tokenizeWords("foo bor baz"))

	if s, ok := d.SliceNew(2, 3); !ok || s != "bor" {
		t.Errorf("SliceNew(2,3) = (%q,%v), want (\"bor\",true)", s, ok)
	}
	got := slices.Collect(d.AllRemappedChanges())
	want := []RemappedChange{
		{diff.ChangeEqual, "foo "},
		{diff.ChangeDelete, "bar"},
		{diff.ChangeInsert, "bor"},
		{diff.ChangeEqual, " baz"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("remapped = %+v, want %+v", got, want)
	}
}

// Joining the tokens reproduces the input byte for byte, so the source a remap
// slices against is the source the diff was built from.
func TestReconstructedSourceMatchesInput(t *testing.T) {
	inputs := []string{
		"foo bar baz",
		"a\nb\r\nc\rd",
		"héllo wörld",
		"",
		"   leading and trailing   ",
	}
	for _, in := range inputs {
		diffs := map[string]*TextDiff{
			"lines": DiffLines(in, in),
			"words": DiffWords(in, in),
			"chars": DiffChars(in, in),
		}
		for name, d := range diffs {
			old, new := d.remap()
			if old.source != in {
				t.Errorf("%s(%q): reconstructed old = %q", name, in, old.source)
			}
			if new.source != in {
				t.Errorf("%s(%q): reconstructed new = %q", name, in, new.source)
			}
		}
	}
}

func TestRemappedChangesPanicsOutOfBounds(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("RemappedChanges with an out-of-range op: want panic, got none")
		}
	}()
	d := DiffWords("foo", "foo")
	d.RemappedChanges(diff.DiffOp{Tag: diff.Equal, OldIndex: 0, NewIndex: 0, OldLen: 99, NewLen: 99})
}
