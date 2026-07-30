package similar

import "github.com/mahibulhaque/similar/internal/diffutil"

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
