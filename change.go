package similar

import (
	"encoding/json"
	"fmt"
)

// ChangeTag identifies the kind of a Change. Unlike DiffTag it has no Replace:
// a replacement is expanded into deletes followed by inserts before it reaches
// a Change.
type ChangeTag int

const (
	// ChangeEqual marks an unchanged value present in both sequences.
	ChangeEqual ChangeTag = iota
	// ChangeDelete marks a value removed from the old sequence.
	ChangeDelete
	// ChangeInsert marks a value added in the new sequence.
	ChangeInsert
)

// String renders the tag as its one-character diff marker: a space for equal,
// '-' for delete, '+' for insert. This mirrors the upstream crate's Display.
func (t ChangeTag) String() string {
	switch t {
	case ChangeEqual:
		return " "
	case ChangeDelete:
		return "-"
	case ChangeInsert:
		return "+"
	default:
		return fmt.Sprintf("ChangeTag(%d)", int(t))
	}
}

// name returns the stable snake-case name used for JSON.
func (t ChangeTag) name() string {
	switch t {
	case ChangeEqual:
		return "equal"
	case ChangeDelete:
		return "delete"
	case ChangeInsert:
		return "insert"
	default:
		return fmt.Sprintf("ChangeTag(%d)", int(t))
	}
}

// MarshalJSON emits the tag as its stable snake-case name (equal/delete/insert),
// matching the upstream serde representation.
func (t ChangeTag) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.name())
}

// UnmarshalJSON parses a snake-case tag name.
func (t *ChangeTag) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "equal":
		*t = ChangeEqual
	case "delete":
		*t = ChangeDelete
	case "insert":
		*t = ChangeInsert
	default:
		return fmt.Errorf("diff: unknown ChangeTag %q", s)
	}
	return nil
}

// Change is an expanded DiffOp: a single tagged value with its position in the
// old and/or new sequence. It is the unit produced when a text diff is walked
// change-by-change.
//
// A ChangeDelete has an old index but no new index; a ChangeInsert has a new
// index but no old index; a ChangeEqual has both. Which indices are present
// follows from the tag, so a Change is built with one constructor per tag and
// absent indices are read back through the comma-ok accessors OldIndex/NewIndex.
type Change struct {
	tag         ChangeTag
	oldIndex    int
	newIndex    int
	hasOldIndex bool
	hasNewIndex bool
	value       string
}

// equalChange builds an unchanged value present in both sequences.
func equalChange(value string, oldIndex, newIndex int) Change {
	return Change{
		tag:         ChangeEqual,
		oldIndex:    oldIndex,
		newIndex:    newIndex,
		hasOldIndex: true,
		hasNewIndex: true,
		value:       value,
	}
}

// deleteChange builds a value removed from the old sequence. It has no new index.
func deleteChange(value string, oldIndex int) Change {
	return Change{
		tag:         ChangeDelete,
		oldIndex:    oldIndex,
		hasOldIndex: true,
		value:       value,
	}
}

// insertChange builds a value added in the new sequence. It has no old index.
func insertChange(value string, newIndex int) Change {
	return Change{
		tag:         ChangeInsert,
		newIndex:    newIndex,
		hasNewIndex: true,
		value:       value,
	}
}

// Tag returns the change tag.
func (c Change) Tag() ChangeTag { return c.tag }

// Value returns the changed value.
func (c Change) Value() string { return c.value }

// OldIndex returns the index in the old sequence and whether it is present.
func (c Change) OldIndex() (int, bool) {
	if !c.hasOldIndex {
		return 0, false
	}
	return c.oldIndex, true
}

// NewIndex returns the index in the new sequence and whether it is present.
func (c Change) NewIndex() (int, bool) {
	if !c.hasNewIndex {
		return 0, false
	}
	return c.newIndex, true
}

// MissingNewline reports whether the value does not end in a newline and would
// need one appended when rendering line-based diffs.
func (c Change) MissingNewline() bool {
	return !endsWithNewline(c.value)
}

// String renders the value, appending a newline when one is missing so that
// printing changes from a line diff yields well-formed lines.
func (c Change) String() string {
	if c.MissingNewline() {
		return c.value + "\n"
	}
	return c.value
}

// MarshalJSON emits the change with snake-case field names; absent indices are
// serialized as null, matching the upstream serde output.
func (c Change) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Tag      ChangeTag `json:"tag"`
		OldIndex *int      `json:"old_index"`
		NewIndex *int      `json:"new_index"`
		Value    string    `json:"value"`
	}{c.tag, optIndex(c.oldIndex, c.hasOldIndex), optIndex(c.newIndex, c.hasNewIndex), c.value})
}

// optIndex renders an optional index as the pointer encoding json needs, so an
// absent index marshals to null. The pointer exists only for serialization.
func optIndex(i int, present bool) *int {
	if !present {
		return nil
	}
	return &i
}

func endsWithNewline(s string) bool {
	if s == "" {
		return false
	}
	last := s[len(s)-1]
	return last == '\n' || last == '\r'
}
