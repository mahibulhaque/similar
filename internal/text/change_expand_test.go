package text

import (
	"reflect"
	"slices"
	"testing"

	"github.com/mahibulhaque/similar/internal/diff"
)

// tagValue is a compact view of a change for comparison.
type tagValue struct {
	tag      diff.ChangeTag
	value    string
	oldIndex int // -1 when absent
	newIndex int // -1 when absent
}

func view(c diff.Change) tagValue {
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

func viewAll(seq func(func(diff.Change) bool)) []tagValue {
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
		{diff.ChangeEqual, "a\n", 0, 0},
		{diff.ChangeEqual, "b\n", 1, 1},
		{diff.ChangeDelete, "c", 2, -1},
		{diff.ChangeInsert, "C", -1, 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changes = %+v, want %+v", got, want)
	}
}

func TestAllChangesCharsReplaceExpands(t *testing.T) {
	d := DiffChars("abcdef", "abcDDf")
	got := viewAll(d.AllChanges())
	want := []tagValue{
		{diff.ChangeEqual, "a", 0, 0},
		{diff.ChangeEqual, "b", 1, 1},
		{diff.ChangeEqual, "c", 2, 2},
		{diff.ChangeDelete, "d", 3, -1},
		{diff.ChangeDelete, "e", 4, -1},
		{diff.ChangeInsert, "D", -1, 3},
		{diff.ChangeInsert, "D", -1, 4},
		{diff.ChangeEqual, "f", 5, 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changes = %+v, want %+v", got, want)
	}
}

func TestChangesSingleOp(t *testing.T) {
	d := DiffChars("abcdef", "abcDDf")
	// The replace op is the second op.
	ops := d.Ops()
	var replace diff.DiffOp
	found := false
	for _, op := range ops {
		if op.Tag == diff.Replace {
			replace = op
			found = true
		}
	}
	if !found {
		t.Fatalf("no replace op in %v", ops)
	}
	got := viewAll(d.Changes(replace))
	want := []tagValue{
		{diff.ChangeDelete, "d", 3, -1},
		{diff.ChangeDelete, "e", 4, -1},
		{diff.ChangeInsert, "D", -1, 3},
		{diff.ChangeInsert, "D", -1, 4},
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
