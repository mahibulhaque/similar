package similar

import (
	"reflect"
	"slices"
	"testing"
)

// tagValue is a compact view of a change for comparison.
type tagValue struct {
	tag      ChangeTag
	value    string
	oldIndex int // -1 when absent
	newIndex int // -1 when absent
}

func view(c Change) tagValue {
	oi, ok := c.OldIndex()
	if !ok {
		oi = -1
	}
	ni, ok := c.NewIndex()
	if !ok {
		ni = -1
	}
	return tagValue{c.Tag(), c.Value(), oi, ni}
}

func viewAll(seq func(func(Change) bool)) []tagValue {
	var out []tagValue
	for c := range seq {
		out = append(out, view(c))
	}
	return out
}

func TestAllChangesLines(t *testing.T) {
	d := DiffLines("a\nb\nc", "a\nb\nC")
	got := viewAll(d.AllChanges())
	want := []tagValue{
		{ChangeEqual, "a\n", 0, 0},
		{ChangeEqual, "b\n", 1, 1},
		{ChangeDelete, "c", 2, -1},
		{ChangeInsert, "C", -1, 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changes = %+v, want %+v", got, want)
	}
}

func TestAllChangesCharsReplaceExpands(t *testing.T) {
	d := DiffChars("abcdef", "abcDDf")
	got := viewAll(d.AllChanges())
	want := []tagValue{
		{ChangeEqual, "a", 0, 0},
		{ChangeEqual, "b", 1, 1},
		{ChangeEqual, "c", 2, 2},
		{ChangeDelete, "d", 3, -1},
		{ChangeDelete, "e", 4, -1},
		{ChangeInsert, "D", -1, 3},
		{ChangeInsert, "D", -1, 4},
		{ChangeEqual, "f", 5, 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changes = %+v, want %+v", got, want)
	}
}

func TestChangesSingleOp(t *testing.T) {
	d := DiffChars("abcdef", "abcDDf")
	// The replace op is the second op.
	ops := d.Ops()
	var replace DiffOp
	found := false
	for _, op := range ops {
		if op.Tag == Replace {
			replace = op
			found = true
		}
	}
	if !found {
		t.Fatalf("no replace op in %v", ops)
	}
	got := viewAll(d.Changes(replace))
	want := []tagValue{
		{ChangeDelete, "d", 3, -1},
		{ChangeDelete, "e", 4, -1},
		{ChangeInsert, "D", -1, 3},
		{ChangeInsert, "D", -1, 4},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changes = %+v, want %+v", got, want)
	}
}

func TestAllChangesEarlyBreak(t *testing.T) {
	d := DiffChars("abcdef", "abcDDf")
	count := 0
	for range d.AllChanges() {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Fatalf("early break yielded %d changes, want 2", count)
	}
}

func TestAllChangesCollect(t *testing.T) {
	d := DiffChars("abcdef", "abcDDf")
	all := slices.Collect(d.AllChanges())
	if len(all) != 8 {
		t.Fatalf("collected %d changes, want 8", len(all))
	}
}
