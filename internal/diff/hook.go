package diff

// DiffHook reacts to an edit script from the old version to the new version.
//
// The algorithm invokes the callbacks as the diff is produced; a hook never
// sees the sequence values, only indices and lengths. Any callback may return
// an error, which aborts the diff and propagates to the caller. Finish is
// always called after the last operation.
//
// Because Go interfaces cannot supply a default method that dispatches back to
// the concrete type, the interface lists every callback. Implementations that
// only care about some callbacks should embed NoopHook and override the rest.
type DiffHook interface {
	// Equal reports that old[oldIndex:oldIndex+length] equals
	// new[newIndex:newIndex+length].
	Equal(oldIndex, newIndex, length int) error
	// Delete reports that old[oldIndex:oldIndex+oldLen] is removed; newIndex
	// is the position in the new sequence at the point of deletion.
	Delete(oldIndex, oldLen, newIndex int) error
	// Insert reports that new[newIndex:newIndex+newLen] is inserted at
	// oldIndex in the old sequence.
	Insert(oldIndex, newIndex, newLen int) error
	// Replace reports that old[oldIndex:oldIndex+oldLen] is replaced by
	// new[newIndex:newIndex+newLen].
	Replace(oldIndex, oldLen, newIndex, newLen int) error
	// Finish is called once after the final operation.
	Finish() error
}

// NoopHook is an embeddable DiffHook whose callbacks all return nil.
//
// Embed it to implement only the callbacks you care about:
//
//	type onlyEqual struct{ diff.NoopHook }
//	func (h *onlyEqual) Equal(o, n, l int) error { /* ... */ return nil }
type NoopHook struct{}

func (NoopHook) Equal(oldIndex, newIndex, length int) error           { return nil }
func (NoopHook) Delete(oldIndex, oldLen, newIndex int) error          { return nil }
func (NoopHook) Insert(oldIndex, newIndex, newLen int) error          { return nil }
func (NoopHook) Replace(oldIndex, oldLen, newIndex, newLen int) error { return nil }
func (NoopHook) Finish() error                                        { return nil }

var _ DiffHook = NoopHook{}
