package similar

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
