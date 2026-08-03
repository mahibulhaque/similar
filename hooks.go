package similar

import "github.com/mahibulhaque/similar/internal/diffutil"

// This file holds the hook contract and every hook this package ships: the
// DiffHook interface and its NoopHook base, then the three implementations the
// standard stack is built from — Capture accumulates, ReplaceHook coalesces an
// adjacent delete+insert into a Replace, and compact cleans the script up
// semantically. engine.go is what assembles them.

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
//	type onlyEqual struct{ similar.NoopHook }
//	func (h *onlyEqual) Equal(o, n, l int) error { /* ... */ return nil }
type NoopHook struct{}

func (NoopHook) Equal(oldIndex, newIndex, length int) error           { return nil }
func (NoopHook) Delete(oldIndex, oldLen, newIndex int) error          { return nil }
func (NoopHook) Insert(oldIndex, newIndex, newLen int) error          { return nil }
func (NoopHook) Replace(oldIndex, oldLen, newIndex, newLen int) error { return nil }
func (NoopHook) Finish() error                                        { return nil }

var _ DiffHook = NoopHook{}

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

// triple is an optional (a, b, c) tuple; ok reports whether it is set.
type triple struct {
	a, b, c int
	ok      bool
}

// ReplaceHook is a DiffHook wrapper that coalesces adjacent deletions and
// insertions into Replace operations (and merges runs of like operations),
// forwarding the result to an inner hook.
//
// The core algorithm emits only equal/delete/insert; wrap it in a ReplaceHook
// to obtain replace semantics without the core producing them. (It is named
// ReplaceHook rather than Replace to avoid clashing with the Replace DiffTag
// constant in this package.)
type ReplaceHook struct {
	d   DiffHook
	del triple // (oldIndex, oldLen, newIndex)
	ins triple // (oldIndex, newIndex, newLen)
	eq  triple // (oldIndex, newIndex, len)
}

// NewReplaceHook wraps an inner hook.
func NewReplaceHook(d DiffHook) *ReplaceHook {
	return &ReplaceHook{d: d}
}

// Inner returns the wrapped hook.
func (r *ReplaceHook) Inner() DiffHook { return r.d }

func (r *ReplaceHook) flushEq() error {
	if r.eq.ok {
		eq := r.eq
		r.eq = triple{}
		return r.d.Equal(eq.a, eq.b, eq.c)
	}
	return nil
}

func (r *ReplaceHook) flushDelIns() error {
	if r.del.ok {
		del := r.del
		r.del = triple{}
		if r.ins.ok {
			ins := r.ins
			r.ins = triple{}
			return r.d.Replace(del.a, del.b, ins.b, ins.c)
		}
		return r.d.Delete(del.a, del.b, del.c)
	}
	if r.ins.ok {
		ins := r.ins
		r.ins = triple{}
		return r.d.Insert(ins.a, ins.b, ins.c)
	}
	return nil
}

func (r *ReplaceHook) Equal(oldIndex, newIndex, length int) error {
	if err := r.flushDelIns(); err != nil {
		return err
	}
	if r.eq.ok {
		r.eq.c += length
	} else {
		r.eq = triple{a: oldIndex, b: newIndex, c: length, ok: true}
	}
	return nil
}

func (r *ReplaceHook) Delete(oldIndex, oldLen, newIndex int) error {
	if err := r.flushEq(); err != nil {
		return err
	}
	if r.del.ok {
		r.del.b += oldLen
	} else {
		r.del = triple{a: oldIndex, b: oldLen, c: newIndex, ok: true}
	}
	return nil
}

func (r *ReplaceHook) Insert(oldIndex, newIndex, newLen int) error {
	if err := r.flushEq(); err != nil {
		return err
	}
	if r.ins.ok {
		r.ins.c += newLen
	} else {
		r.ins = triple{a: oldIndex, b: newIndex, c: newLen, ok: true}
	}
	return nil
}

func (r *ReplaceHook) Replace(oldIndex, oldLen, newIndex, newLen int) error {
	if err := r.flushEq(); err != nil {
		return err
	}
	return r.d.Replace(oldIndex, oldLen, newIndex, newLen)
}

func (r *ReplaceHook) Finish() error {
	if err := r.flushEq(); err != nil {
		return err
	}
	if err := r.flushDelIns(); err != nil {
		return err
	}
	return r.d.Finish()
}

var _ DiffHook = (*ReplaceHook)(nil)

// compact is a DiffHook that performs semantic cleanup on a diff before
// forwarding it to an inner hook. It records equal/delete/insert operations,
// then on Finish shifts and merges adjacent hunks to connect as many changes
// as possible, and finally replays the cleaned ops to the inner hook.
//
// It is based on the compaction logic from diffy by Brandon Williams and still
// needs to be combined with Replace to emit actual Replace operations.
type compact[T comparable] struct {
	d   DiffHook
	ops []DiffOp
	old []T
	new []T
}

// newCompact wraps an inner hook, capturing against old and new for cleanup.
func newCompact[T comparable](d DiffHook, old, new []T) *compact[T] {
	return &compact[T]{d: d, old: old, new: new}
}

// Inner returns the wrapped hook.
func (c *compact[T]) Inner() DiffHook { return c.d }

func (c *compact[T]) Equal(oldIndex, newIndex, length int) error {
	c.ops = append(c.ops, DiffOp{Tag: Equal, OldIndex: oldIndex, NewIndex: newIndex, OldLen: length, NewLen: length})
	return nil
}

func (c *compact[T]) Delete(oldIndex, oldLen, newIndex int) error {
	c.ops = append(c.ops, DiffOp{Tag: Delete, OldIndex: oldIndex, NewIndex: newIndex, OldLen: oldLen})
	return nil
}

func (c *compact[T]) Insert(oldIndex, newIndex, newLen int) error {
	c.ops = append(c.ops, DiffOp{Tag: Insert, OldIndex: oldIndex, NewIndex: newIndex, NewLen: newLen})
	return nil
}

func (c *compact[T]) Replace(oldIndex, oldLen, newIndex, newLen int) error {
	c.ops = append(c.ops, DiffOp{Tag: Replace, OldIndex: oldIndex, NewIndex: newIndex, OldLen: oldLen, NewLen: newLen})
	return nil
}

func (c *compact[T]) Finish() error {
	c.cleanup()
	for i := range c.ops {
		if err := c.ops[i].ApplyToHook(c.d); err != nil {
			return err
		}
	}
	return c.d.Finish()
}

var _ DiffHook = (*compact[int])(nil)

func insertOp(ops []DiffOp, at int, op DiffOp) []DiffOp {
	ops = append(ops, DiffOp{})
	copy(ops[at+1:], ops[at:])
	ops[at] = op
	return ops
}

func removeOp(ops []DiffOp, at int) []DiffOp {
	return append(ops[:at], ops[at+1:]...)
}

// cleanup walks all edits shifting them up then down, merging where they meet
// similar edits.
func (c *compact[T]) cleanup() {
	// First compact deletions.
	pointer := 0
	for pointer < len(c.ops) {
		if c.ops[pointer].Tag == Delete {
			pointer = c.shiftUp(pointer)
			pointer = c.shiftDown(pointer)
		}
		pointer++
	}

	// Then compact insertions.
	pointer = 0
	for pointer < len(c.ops) {
		if c.ops[pointer].Tag == Insert {
			pointer = c.shiftUp(pointer)
			pointer = c.shiftDown(pointer)
		}
		pointer++
	}

	c.normalizeCursors()
}

func (c *compact[T]) normalizeCursors() {
	oldCursor, newCursor := 0, 0
	if len(c.ops) > 0 {
		oldCursor = c.ops[0].OldIndex
		newCursor = c.ops[0].NewIndex
	}
	for i := range c.ops {
		op := &c.ops[i]
		switch op.Tag {
		case Equal:
			oldCursor = op.OldIndex + op.OldLen
			newCursor = op.NewIndex + op.NewLen
		case Delete:
			op.NewIndex = newCursor
			oldCursor = op.OldIndex + op.OldLen
		case Insert:
			op.OldIndex = oldCursor
			newCursor = op.NewIndex + op.NewLen
		case Replace:
			oldCursor = op.OldIndex + op.OldLen
			newCursor = op.NewIndex + op.NewLen
		}
	}
}

// swapAdjacentInsertDelete swaps ops[left] and ops[left+1] (an insert/delete
// pair) patching cursor positions so the stream stays contiguous.
func (c *compact[T]) swapAdjacentInsertDelete(left int) {
	right := left + 1
	oldStart := c.ops[left].OldIndex
	oldEnd := c.ops[right].OldIndex + c.ops[right].OldLen
	newStart := c.ops[left].NewIndex
	newEnd := c.ops[right].NewIndex + c.ops[right].NewLen

	c.ops[left], c.ops[right] = c.ops[right], c.ops[left]

	switch c.ops[left].Tag {
	case Insert:
		c.ops[left].OldIndex = oldStart
	case Delete:
		c.ops[left].NewIndex = newStart
	}
	switch c.ops[right].Tag {
	case Insert:
		c.ops[right].OldIndex = oldEnd
	case Delete:
		c.ops[right].NewIndex = newEnd
	}
}

func (c *compact[T]) shiftUp(pointer int) int {
	for pointer >= 1 {
		prevOp := c.ops[pointer-1]
		thisOp := c.ops[pointer]
		switch {
		case thisOp.Tag == Insert && prevOp.Tag == Equal:
			ps, pe := prevOp.OldRange()
			ns, ne := thisOp.NewRange()
			suffixLen := diffutil.CommonSuffixLen(c.old, ps, pe, c.new, ns, ne)
			if suffixLen > 0 {
				if pointer+1 < len(c.ops) && c.ops[pointer+1].Tag == Equal {
					c.ops[pointer+1].growLeft(suffixLen)
				} else {
					c.ops = insertOp(c.ops, pointer+1, DiffOp{
						Tag:      Equal,
						OldIndex: pe - suffixLen,
						NewIndex: ne - suffixLen,
						OldLen:   suffixLen,
						NewLen:   suffixLen,
					})
				}
				c.ops[pointer].shiftLeft(suffixLen)
				c.ops[pointer-1].shrinkLeft(suffixLen)
				if c.ops[pointer-1].IsEmpty() {
					c.ops = removeOp(c.ops, pointer-1)
					pointer--
				}
			} else if c.ops[pointer-1].IsEmpty() {
				c.ops = removeOp(c.ops, pointer-1)
				pointer--
			} else {
				return pointer
			}
		case thisOp.Tag == Delete && prevOp.Tag == Equal:
			canMerge := pointer >= 2 && c.ops[pointer-2].Tag == Delete
			if !canMerge {
				return pointer
			}
			ts, te := thisOp.OldRange()
			ns, ne := prevOp.NewRange()
			suffixLen := diffutil.CommonSuffixLen(c.old, ts, te, c.new, ns, ne)
			if suffixLen != 0 {
				if pointer+1 < len(c.ops) && c.ops[pointer+1].Tag == Equal {
					c.ops[pointer+1].growLeft(suffixLen)
				} else {
					c.ops = insertOp(c.ops, pointer+1, DiffOp{
						Tag:      Equal,
						OldIndex: te - suffixLen,
						NewIndex: ne - suffixLen,
						OldLen:   suffixLen,
						NewLen:   suffixLen,
					})
				}
				c.ops[pointer].shiftLeft(suffixLen)
				c.ops[pointer-1].shrinkLeft(suffixLen)
				if c.ops[pointer-1].IsEmpty() {
					c.ops = removeOp(c.ops, pointer-1)
					pointer--
				}
			} else if c.ops[pointer-1].IsEmpty() {
				c.ops = removeOp(c.ops, pointer-1)
				pointer--
			} else {
				return pointer
			}
		case (thisOp.Tag == Insert && prevOp.Tag == Delete) ||
			(thisOp.Tag == Delete && prevOp.Tag == Insert):
			c.swapAdjacentInsertDelete(pointer - 1)
			pointer--
		case thisOp.Tag == Insert && prevOp.Tag == Insert:
			_, ne := thisOp.NewRange()
			ns, _ := thisOp.NewRange()
			c.ops[pointer-1].growRight(ne - ns)
			c.ops = removeOp(c.ops, pointer)
			pointer--
		case thisOp.Tag == Delete && prevOp.Tag == Delete:
			os, oe := thisOp.OldRange()
			c.ops[pointer-1].growRight(oe - os)
			c.ops = removeOp(c.ops, pointer)
			pointer--
		default:
			return pointer
		}
	}
	return pointer
}

func (c *compact[T]) shiftDown(pointer int) int {
	for pointer+1 < len(c.ops) {
		nextOp := c.ops[pointer+1]
		thisOp := c.ops[pointer]
		switch {
		case thisOp.Tag == Insert && nextOp.Tag == Equal:
			os, oe := nextOp.OldRange()
			ns, ne := thisOp.NewRange()
			prefixLen := diffutil.CommonPrefixLen(c.old, os, oe, c.new, ns, ne)
			if prefixLen > 0 {
				if pointer >= 1 && c.ops[pointer-1].Tag == Equal {
					c.ops[pointer-1].growRight(prefixLen)
				} else {
					c.ops = insertOp(c.ops, pointer, DiffOp{
						Tag:      Equal,
						OldIndex: os,
						NewIndex: ns,
						OldLen:   prefixLen,
						NewLen:   prefixLen,
					})
					pointer++
				}
				c.ops[pointer].shiftRight(prefixLen)
				c.ops[pointer+1].shrinkRight(prefixLen)
				if c.ops[pointer+1].IsEmpty() {
					c.ops = removeOp(c.ops, pointer+1)
				}
			} else if c.ops[pointer+1].IsEmpty() {
				c.ops = removeOp(c.ops, pointer+1)
			} else {
				return pointer
			}
		case thisOp.Tag == Delete && nextOp.Tag == Equal:
			canMerge := pointer+2 < len(c.ops) && c.ops[pointer+2].Tag == Delete
			if !canMerge {
				return pointer
			}
			os, oe := thisOp.OldRange()
			ns, ne := nextOp.NewRange()
			prefixLen := diffutil.CommonPrefixLen(c.old, os, oe, c.new, ns, ne)
			if prefixLen > 0 {
				if pointer >= 1 && c.ops[pointer-1].Tag == Equal {
					c.ops[pointer-1].growRight(prefixLen)
				} else {
					c.ops = insertOp(c.ops, pointer, DiffOp{
						Tag:      Equal,
						OldIndex: os,
						NewIndex: thisOp.NewIndex,
						OldLen:   prefixLen,
						NewLen:   prefixLen,
					})
					pointer++
				}
				c.ops[pointer].shiftRight(prefixLen)
				c.ops[pointer+1].shrinkRight(prefixLen)
				if c.ops[pointer+1].IsEmpty() {
					c.ops = removeOp(c.ops, pointer+1)
				}
			} else if c.ops[pointer+1].IsEmpty() {
				c.ops = removeOp(c.ops, pointer+1)
			} else {
				return pointer
			}
		case (thisOp.Tag == Insert && nextOp.Tag == Delete) ||
			(thisOp.Tag == Delete && nextOp.Tag == Insert):
			c.swapAdjacentInsertDelete(pointer)
			pointer++
		case thisOp.Tag == Insert && nextOp.Tag == Insert:
			ns, ne := nextOp.NewRange()
			c.ops[pointer].growRight(ne - ns)
			c.ops = removeOp(c.ops, pointer+1)
		case thisOp.Tag == Delete && nextOp.Tag == Delete:
			os, oe := nextOp.OldRange()
			c.ops[pointer].growRight(oe - os)
			c.ops = removeOp(c.ops, pointer+1)
		default:
			return pointer
		}
	}
	return pointer
}
