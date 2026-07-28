package text

import (
	"strings"

	"github.com/mahibulhaque/similar/internal/diff"
)

// RemappedChange is a tagged run of the original text: the value is a connected
// substring of the old or new input, not a single token.
type RemappedChange struct {
	Tag   diff.ChangeTag
	Value string
}

// sideRemapper reconstructs one side's source text from its tokens and maps a
// half-open token range onto a substring of it.
//
// The tokens are the only source of truth. Every tokenizer here partitions its
// input, so joining them reproduces the text the diff was built from and no
// caller-supplied string can disagree with the token offsets.
//
// starts has one more entry than there are tokens: starts[i] is token i's byte
// offset and starts[len(tokens)] is the total length, so mapping a range needs
// no second table.
type sideRemapper struct {
	source string
	starts []int
}

// newSideRemapper joins the tokens and records where each one begins.
func newSideRemapper(tokens []string) sideRemapper {
	var b strings.Builder
	starts := make([]int, len(tokens)+1)
	for i, t := range tokens {
		starts[i] = b.Len()
		b.WriteString(t)
	}
	starts[len(tokens)] = b.Len()
	return sideRemapper{source: b.String(), starts: starts}
}

// slice returns the source substring covered by token indices [start, end) and
// whether the range is valid. An empty range is valid and yields "". The result
// shares the source's backing array, so no bytes are copied.
func (r sideRemapper) slice(start, end int) (string, bool) {
	if start < 0 || end < start || end >= len(r.starts) {
		return "", false
	}
	return r.source[r.starts[start]:r.starts[end]], true
}
