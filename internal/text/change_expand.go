package text

import (
	"iter"

	"github.com/mahibulhaque/similar/internal/diff"
)

// Changes returns the changes a single op expands to. An Equal, Delete, or
// Insert maps directly; a Replace expands to all its deletes followed by all
// its inserts, so a Change only ever carries the equal/delete/insert tags.
//
// It panics if op holds indices out of range for this diff's tokens, matching
// the upstream crate.
func (d *TextDiff) Changes(op diff.DiffOp) iter.Seq[diff.Change] {
	return func(yield func(diff.Change) bool) {
		d.emitChanges(op, yield)
	}
}

// AllChanges flattens every op into a single lazy stream of changes.
func (d *TextDiff) AllChanges() iter.Seq[diff.Change] {
	return func(yield func(diff.Change) bool) {
		for _, op := range d.ops {
			if !d.emitChanges(op, yield) {
				return
			}
		}
	}
}

// emitChanges yields the changes for op and reports whether iteration should
// continue (false once yield has asked to stop).
func (d *TextDiff) emitChanges(op diff.DiffOp, yield func(diff.Change) bool) bool {
	switch op.Tag {
	case diff.Equal:
		for k := 0; k < op.OldLen; k++ {
			oi, ni := op.OldIndex+k, op.NewIndex+k
			if !yield(diff.EqualChange(d.old[oi], oi, ni)) {
				return false
			}
		}
	case diff.Delete:
		for k := 0; k < op.OldLen; k++ {
			oi := op.OldIndex + k
			if !yield(diff.DeleteChange(d.old[oi], oi)) {
				return false
			}
		}
	case diff.Insert:
		for k := 0; k < op.NewLen; k++ {
			ni := op.NewIndex + k
			if !yield(diff.InsertChange(d.new[ni], ni)) {
				return false
			}
		}
	case diff.Replace:
		for k := 0; k < op.OldLen; k++ {
			oi := op.OldIndex + k
			if !yield(diff.DeleteChange(d.old[oi], oi)) {
				return false
			}
		}
		for k := 0; k < op.NewLen; k++ {
			ni := op.NewIndex + k
			if !yield(diff.InsertChange(d.new[ni], ni)) {
				return false
			}
		}
	}
	return true
}
