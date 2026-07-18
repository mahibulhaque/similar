// Package diff holds the canonical, algorithm-agnostic diff vocabulary: the
// tagged DiffOp operation, the DiffHook streaming interface, the embeddable
// NoopHook, and the Capture / Replace / Compact hooks.
//
// It is a leaf package (standard library only) so that the algorithm packages
// and the public facade can share one operation type without an import cycle.
package diff

import (
	"encoding/json"
	"fmt"
)

// DiffTag identifies the kind of a DiffOp.
type DiffTag int

const (
	// Equal marks a span that is identical in both sequences.
	Equal DiffTag = iota
	// Delete marks a span removed from the old sequence.
	Delete
	// Insert marks a span added from the new sequence.
	Insert
	// Replace marks a span of the old sequence replaced by a span of the new.
	Replace
)

// String returns the stable snake-case name of the tag.
func (t DiffTag) String() string {
	switch t {
	case Equal:
		return "equal"
	case Delete:
		return "delete"
	case Insert:
		return "insert"
	case Replace:
		return "replace"
	default:
		return fmt.Sprintf("DiffTag(%d)", int(t))
	}
}

// MarshalJSON emits the tag as its stable snake-case name.
func (t DiffTag) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// UnmarshalJSON parses a snake-case tag name.
func (t *DiffTag) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "equal":
		*t = Equal
	case "delete":
		*t = Delete
	case "insert":
		*t = Insert
	case "replace":
		*t = Replace
	default:
		return fmt.Errorf("diff: unknown DiffTag %q", s)
	}
	return nil
}

// DiffOp is a single tagged diff operation carrying indices and lengths into
// the original sequences.
//
// For an Equal, OldLen == NewLen == the span length. For a Delete, NewLen is
// zero; for an Insert, OldLen is zero. This uniform (index, len) layout lets
// OldRange and NewRange be computed the same way for every tag.
type DiffOp struct {
	Tag      DiffTag `json:"tag"`
	OldIndex int     `json:"old_index"`
	NewIndex int     `json:"new_index"`
	OldLen   int     `json:"old_len,omitempty"`
	NewLen   int     `json:"new_len,omitempty"`
}

// Len returns the length of an Equal span. For an Equal, OldLen == NewLen.
func (op DiffOp) Len() int {
	return op.OldLen
}

// OldRange returns the half-open range [start, end) the op covers in the old
// sequence.
func (op DiffOp) OldRange() (start, end int) {
	return op.OldIndex, op.OldIndex + op.OldLen
}

// NewRange returns the half-open range [start, end) the op covers in the new
// sequence.
func (op DiffOp) NewRange() (start, end int) {
	return op.NewIndex, op.NewIndex + op.NewLen
}

// IsEmpty reports whether the op covers nothing on either side.
func (op DiffOp) IsEmpty() bool {
	return op.OldLen == 0 && op.NewLen == 0
}

// ApplyToHook dispatches the op to the matching hook callback.
func (op DiffOp) ApplyToHook(d DiffHook) error {
	switch op.Tag {
	case Equal:
		return d.Equal(op.OldIndex, op.NewIndex, op.OldLen)
	case Delete:
		return d.Delete(op.OldIndex, op.OldLen, op.NewIndex)
	case Insert:
		return d.Insert(op.OldIndex, op.NewIndex, op.NewLen)
	case Replace:
		return d.Replace(op.OldIndex, op.OldLen, op.NewIndex, op.NewLen)
	default:
		return fmt.Errorf("diff: invalid DiffOp tag %d", op.Tag)
	}
}

// adjust shifts the op's start offsets by offAdj and its length(s) by lenAdj.
// The length adjustment lands on the field(s) meaningful for the tag: OldLen
// for Delete, NewLen for Insert, both for Equal and Replace.
func (op *DiffOp) adjust(offAdj, lenAdj int) {
	op.OldIndex += offAdj
	op.NewIndex += offAdj
	switch op.Tag {
	case Equal, Replace:
		op.OldLen += lenAdj
		op.NewLen += lenAdj
	case Delete:
		op.OldLen += lenAdj
	case Insert:
		op.NewLen += lenAdj
	}
}

func (op *DiffOp) shiftLeft(adjust int)   { op.adjust(-adjust, 0) }
func (op *DiffOp) shiftRight(adjust int)  { op.adjust(adjust, 0) }
func (op *DiffOp) growLeft(adjust int)    { op.adjust(-adjust, adjust) }
func (op *DiffOp) growRight(adjust int)   { op.adjust(0, adjust) }
func (op *DiffOp) shrinkLeft(adjust int)  { op.adjust(0, -adjust) }
func (op *DiffOp) shrinkRight(adjust int) { op.adjust(adjust, -adjust) }
