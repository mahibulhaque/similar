package algorithms

import "fmt"

// The tests judge Myers output as a list of operations, but this package never
// builds one: it only calls diffHook callbacks. These types are that
// materialization, kept test-local so the unit tests depend on the shape of the
// callbacks under test rather than on whichever operation type a consumer
// happens to assemble from them.

// opTag identifies the kind of a capturedOp.
type opTag int

const (
	tagEqual opTag = iota
	tagDelete
	tagInsert
	tagReplace
)

// String keeps failure messages readable, naming the tag rather than its ordinal.
func (t opTag) String() string {
	switch t {
	case tagEqual:
		return "equal"
	case tagDelete:
		return "delete"
	case tagInsert:
		return "insert"
	case tagReplace:
		return "replace"
	default:
		return fmt.Sprintf("opTag(%d)", int(t))
	}
}

// capturedOp is one callback recorded as a tagged span. For an Equal, OldLen and
// NewLen are both the span length; a Delete leaves NewLen zero and an Insert
// leaves OldLen zero, so the ranges are computed the same way for every tag.
type capturedOp struct {
	Tag      opTag
	OldIndex int
	NewIndex int
	OldLen   int
	NewLen   int
}

// OldRange returns the half-open range the op covers in the old sequence.
func (o capturedOp) OldRange() (start, end int) { return o.OldIndex, o.OldIndex + o.OldLen }

// NewRange returns the half-open range the op covers in the new sequence.
func (o capturedOp) NewRange() (start, end int) { return o.NewIndex, o.NewIndex + o.NewLen }

// capture is a diffHook that records every callback in order.
type capture struct {
	ops []capturedOp
}

func newCapture() *capture { return &capture{} }

// Ops returns the recorded operations.
func (c *capture) Ops() []capturedOp { return c.ops }

func (c *capture) Equal(oldIndex, newIndex, length int) error {
	c.ops = append(c.ops, capturedOp{tagEqual, oldIndex, newIndex, length, length})
	return nil
}

func (c *capture) Delete(oldIndex, oldLen, newIndex int) error {
	c.ops = append(c.ops, capturedOp{Tag: tagDelete, OldIndex: oldIndex, NewIndex: newIndex, OldLen: oldLen})
	return nil
}

func (c *capture) Insert(oldIndex, newIndex, newLen int) error {
	c.ops = append(c.ops, capturedOp{Tag: tagInsert, OldIndex: oldIndex, NewIndex: newIndex, NewLen: newLen})
	return nil
}

func (c *capture) Replace(oldIndex, oldLen, newIndex, newLen int) error {
	c.ops = append(c.ops, capturedOp{tagReplace, oldIndex, newIndex, oldLen, newLen})
	return nil
}

func (c *capture) Finish() error { return nil }

// noopHook is an embeddable diffHook whose callbacks all return nil, so a test
// hook can override only the callback it cares about.
type noopHook struct{}

func (noopHook) Equal(oldIndex, newIndex, length int) error           { return nil }
func (noopHook) Delete(oldIndex, oldLen, newIndex int) error          { return nil }
func (noopHook) Insert(oldIndex, newIndex, newLen int) error          { return nil }
func (noopHook) Replace(oldIndex, oldLen, newIndex, newLen int) error { return nil }
func (noopHook) Finish() error                                        { return nil }

var (
	_ diffHook = (*capture)(nil)
	_ diffHook = noopHook{}
)
