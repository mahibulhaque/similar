package text

import (
	"reflect"
	"testing"

	"github.com/mahibulhaque/similar/internal/diff"
)

func TestRemapperWordDiff(t *testing.T) {
	old := "foo bar baz"
	new := "foo bor baz"
	d := DiffWords(old, new)
	rm := NewTextDiffRemapper(d, old, new)

	var got []RemappedChange
	for _, op := range d.Ops() {
		got = append(got, rm.IterSlices(op)...)
	}
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

func TestRemapperCharDiffConnectsRuns(t *testing.T) {
	old := "foobarbaz"
	new := "fooBARbaz"
	d := DiffChars(old, new)
	rm := NewTextDiffRemapper(d, old, new)

	var got []RemappedChange
	for _, op := range d.Ops() {
		got = append(got, rm.IterSlices(op)...)
	}
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

func TestRemapperSliceBounds(t *testing.T) {
	old := "foo bar baz"
	new := "foo bor baz"
	d := DiffWords(old, new)
	rm := NewTextDiffRemapper(d, old, new)

	if s, ok := rm.SliceOld(0, 2); !ok || s != "foo " {
		t.Errorf("SliceOld(0,2) = (%q,%v), want (\"foo \",true)", s, ok)
	}
	if _, ok := rm.SliceOld(0, 100); ok {
		t.Errorf("SliceOld out of range should report !ok")
	}
}

func TestRemapperFromTokens(t *testing.T) {
	oldTokens := tokenizeWords("foo bar baz")
	newTokens := tokenizeWords("foo bor baz")
	rm := NewTextDiffRemapperFromTokens(oldTokens, newTokens, "foo bar baz", "foo bor baz")
	if s, ok := rm.SliceNew(2, 3); !ok || s != "bor" {
		t.Errorf("SliceNew(2,3) = (%q,%v), want (\"bor\",true)", s, ok)
	}
}
