package similar

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// The benchmarks here exist to keep the cost of GetCloseMatches visible. It is
// the one entry point that runs a diff per candidate, so a change that looks
// free in a single diff can be multiplied by the size of the candidate list.
//
// Run with:
//
//	go test -run=NONE -bench=CloseMatches -benchmem .

func benchCandidates(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("identifier_%d_name", i))
	}
	return out
}

// Most candidates survive the prefilters and are scored by a full diff: the
// worst case for a homogeneous candidate list.
func BenchmarkGetCloseMatchesSimilarCandidates(b *testing.B) {
	for _, n := range []int{100, 1000, 5000} {
		candidates := benchCandidates(n)
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			for b.Loop() {
				_ = GetCloseMatches("identifier_2500_nmae", candidates, 3, 0.6)
			}
		})
	}
}

// Almost every candidate is rejected on length alone, which should not cost an
// allocation per candidate.
func BenchmarkGetCloseMatchesRejectedOnLength(b *testing.B) {
	candidates := make([]string, 0, 5000)
	for i := 0; i < 5000; i++ {
		candidates = append(candidates, strings.Repeat("x", 200+i%50))
	}
	b.Run("5000", func(b *testing.B) {
		for b.Loop() {
			_ = GetCloseMatches("short", candidates, 3, 0.6)
		}
	})
}

// The multiset prefilter alone, without the diff behind it.
func BenchmarkQuickSeqRatio(b *testing.B) {
	q := newQuickSeqRatio(tokenizeChars("identifier_2500_nmae"))
	seq := tokenizeChars("identifier_2500_name")
	for b.Loop() {
		_ = q.calc(seq)
	}
}

// captureRatio against the standard stack, on the shape GetCloseMatches uses:
// two short token sequences. The gap between these two is the saving.
func BenchmarkRatioPaths(b *testing.B) {
	old := tokenizeChars("identifier_2500_nmae")
	new := tokenizeChars("identifier_2500_name")

	b.Run("counter", func(b *testing.B) {
		for b.Loop() {
			_ = captureRatio(context.Background(), Myers, old, new)
		}
	})
	b.Run("standard stack", func(b *testing.B) {
		for b.Loop() {
			_ = DiffSlices(old, new, PlainTokens).Ratio()
		}
	})
}
