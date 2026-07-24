package text

import "sort"

// GetCloseMatches returns up to n of the possibilities whose character-level
// similarity to word is at least cutoff, ordered by descending similarity with
// a lexicographic tie-break. It mirrors Python's difflib.get_close_matches.
//
// Candidates are cheaply prefiltered with an upper-bound ratio before the full
// character diff is run, so obvious non-matches are skipped.
func GetCloseMatches(word string, possibilities []string, n int, cutoff float64) []string {
	seq1 := tokenizeChars(word)
	quick := newQuickSeqRatio(seq1)

	type scored struct {
		ratio float64
		word  string
	}
	var matches []scored
	for _, p := range possibilities {
		seq2 := tokenizeChars(p)
		if upperSeqRatio(seq1, seq2) < cutoff || quick.calc(seq2) < cutoff {
			continue
		}
		ratio := DiffSlices(seq1, seq2).Ratio()
		if ratio >= cutoff {
			matches = append(matches, scored{ratio: ratio, word: p})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].ratio != matches[j].ratio {
			return matches[i].ratio > matches[j].ratio
		}
		return matches[i].word < matches[j].word
	})

	if n < 0 {
		n = 0
	}
	rv := make([]string, 0, min(n, len(matches)))
	for i := 0; i < len(matches) && i < n; i++ {
		rv = append(rv, matches[i].word)
	}
	return rv
}
