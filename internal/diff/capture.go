package diff

// Capture is a DiffHook that accumulates every operation into a slice.
//
// It is the instrument the library and its tests share for the common case of
// materializing a diff. The zero value is ready to use.
type Capture struct {
	ops []DiffOp
}

// NewCapture returns an empty Capture.
func NewCapture() *Capture {
	return &Capture{}
}

// Ops returns the accumulated operations.
func (c *Capture) Ops() []DiffOp {
	return c.ops
}

func (c *Capture) Equal(oldIndex, newIndex, length int) error {
	c.ops = append(c.ops, DiffOp{
		Tag:      Equal,
		OldIndex: oldIndex,
		NewIndex: newIndex,
		OldLen:   length,
		NewLen:   length,
	})
	return nil
}

func (c *Capture) Delete(oldIndex, oldLen, newIndex int) error {
	c.ops = append(c.ops, DiffOp{
		Tag:      Delete,
		OldIndex: oldIndex,
		NewIndex: newIndex,
		OldLen:   oldLen,
	})
	return nil
}

func (c *Capture) Insert(oldIndex, newIndex, newLen int) error {
	c.ops = append(c.ops, DiffOp{
		Tag:      Insert,
		OldIndex: oldIndex,
		NewIndex: newIndex,
		NewLen:   newLen,
	})
	return nil
}

func (c *Capture) Replace(oldIndex, oldLen, newIndex, newLen int) error {
	c.ops = append(c.ops, DiffOp{
		Tag:      Replace,
		OldIndex: oldIndex,
		NewIndex: newIndex,
		OldLen:   oldLen,
		NewLen:   newLen,
	})
	return nil
}

func (c *Capture) Finish() error { return nil }

var _ DiffHook = (*Capture)(nil)
