package similar

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
