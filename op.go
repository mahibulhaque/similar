// This file holds the algorithm-agnostic diff vocabulary: the tagged DiffOp
// operation, the tag that classifies it, and the two derivations that read a
// captured script — DiffRatio and GroupDiffOps. The DiffHook streaming
// interface and the hooks built on it are in hooks.go.

package similar

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

// DiffRatio returns a measure of the two sequences' similarity in the range
// [0,1], where 1.0 means identical and 0.0 means completely distinct.
//
// It is computed from the captured ops and the sequence lengths as
// 2*matches/(oldLen+newLen), where matches is the total length of the Equal
// spans. When both lengths are zero the sequences are considered identical and
// the ratio is 1.0.
func DiffRatio(ops []DiffOp, oldLen, newLen int) float64 {
	matches := 0
	for _, op := range ops {
		if op.Tag == Equal {
			matches += op.Len()
		}
	}
	total := oldLen + newLen
	if total == 0 {
		return 1.0
	}
	return 2.0 * float64(matches) / float64(total)
}

// GroupDiffOps isolates change clusters by eliminating long runs of equal
// content, leaving n ops of context around each change. It returns one group
// per cluster — the shape a unified diff consumes.
//
// Leading and trailing equal runs are trimmed to at most n context items, and a
// group is split whenever an interior equal run is longer than 2*n (so the two
// changes it separates land in different groups, each keeping n context). A
// trailing group that is empty or a lone equal run is dropped.
//
// The input slice is not modified; groups reference freshly built ops.
func GroupDiffOps(ops []DiffOp, n int) [][]DiffOp {
	if len(ops) == 0 {
		return nil
	}

	work := make([]DiffOp, len(ops))
	copy(work, ops)

	// Trim the leading equal run to n context items.
	if work[0].Tag == Equal {
		offset := satSub(work[0].Len(), n)
		work[0].OldIndex += offset
		work[0].NewIndex += offset
		work[0].OldLen -= offset
		work[0].NewLen -= offset
	}

	// Trim the trailing equal run to n context items.
	last := len(work) - 1
	if work[last].Tag == Equal {
		trim := satSub(work[last].Len(), n)
		work[last].OldLen -= trim
		work[last].NewLen -= trim
	}

	var rv [][]DiffOp
	var pending []DiffOp
	for _, op := range work {
		if op.Tag == Equal && op.Len() > n*2 {
			l := op.Len()
			// Close the current group with n items of trailing context.
			pending = append(pending, DiffOp{
				Tag: Equal, OldIndex: op.OldIndex, NewIndex: op.NewIndex,
				OldLen: n, NewLen: n,
			})
			rv = append(rv, pending)
			// Start the next group with n items of leading context.
			offset := satSub(l, n)
			pending = []DiffOp{{
				Tag:      Equal,
				OldIndex: op.OldIndex + offset,
				NewIndex: op.NewIndex + offset,
				OldLen:   l - offset,
				NewLen:   l - offset,
			}}
			continue
		}
		pending = append(pending, op)
	}

	// Drop a trailing group that carries no change (empty or a lone equal run).
	if len(pending) != 0 && (len(pending) != 1 || pending[0].Tag != Equal) {
		rv = append(rv, pending)
	}
	return rv
}

// satSub is a saturating subtraction that clamps at zero.
func satSub(a, b int) int {
	if a < b {
		return 0
	}
	return a - b
}
