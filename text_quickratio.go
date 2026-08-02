package similar

import "maps"

// upperSeqRatio is a cheap upper bound on the similarity of two token
// sequences: 2*min(len)/ (len1+len2), or 1.0 when both are empty. It is used to
// discard obvious non-matches before running a full diff.
func upperSeqRatio(seq1, seq2 []string) float64 {
	n := len(seq1) + len(seq2)
	if n == 0 {
		return 1.0
	}
	min := len(seq1)
	if len(seq2) < min {
		min = len(seq2)
	}
	return 2.0 * float64(min) / float64(n)
}

// quickSeqRatio computes an order-independent upper-bound ratio by treating the
// sequences as multisets, following Python's difflib. Because Go strings are
// comparable, counting uses a plain map rather than a hashed bucket table.
type quickSeqRatio struct {
	counts map[string]int
	unique int
}

func newQuickSeqRatio(seq []string) quickSeqRatio {
	counts := make(map[string]int, len(seq))
	unique := 0
	for _, w := range seq {
		if counts[w] == 0 {
			unique++
		}
		counts[w]++
	}
	return quickSeqRatio{counts: counts, unique: unique}
}

// calc returns the multiset match ratio of seq against the sequence this ratio
// was built from.
func (q quickSeqRatio) calc(seq []string) float64 {
	n := q.unique + len(seq)
	if n == 0 {
		return 1.0
	}
	avail := maps.Clone(q.counts)
	if avail == nil {
		avail = make(map[string]int)
	}
	matches := 0
	for _, w := range seq {
		if avail[w] > 0 {
			matches++
		}
		avail[w]--
	}
	return 2.0 * float64(matches) / float64(n)
}
