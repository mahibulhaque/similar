package similar

import "testing"

func TestDiffRatio(t *testing.T) {
	cases := []struct {
		name           string
		ops            []DiffOp
		oldLen, newLen int
		want           float64
	}{
		{
			// "abcd" vs "bcde": LCS "bcd" (3), so 2*3/(4+4) = 0.75.
			name: "chars abcd bcde",
			ops: []DiffOp{
				{Tag: Delete, OldIndex: 0, NewIndex: 0, OldLen: 1},
				{Tag: Equal, OldIndex: 1, NewIndex: 0, OldLen: 3, NewLen: 3},
				{Tag: Insert, OldIndex: 4, NewIndex: 3, NewLen: 1},
			},
			oldLen: 4, newLen: 4,
			want: 0.75,
		},
		{
			name:   "both empty is identical",
			ops:    nil,
			oldLen: 0, newLen: 0,
			want: 1.0,
		},
		{
			name: "no equal spans",
			ops: []DiffOp{
				{Tag: Delete, OldIndex: 0, NewIndex: 0, OldLen: 3},
				{Tag: Insert, OldIndex: 3, NewIndex: 0, NewLen: 2},
			},
			oldLen: 3, newLen: 2,
			want: 0.0,
		},
		{
			name: "fully equal",
			ops: []DiffOp{
				{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 5, NewLen: 5},
			},
			oldLen: 5, newLen: 5,
			want: 1.0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DiffRatio(c.ops, c.oldLen, c.newLen); got != c.want {
				t.Errorf("DiffRatio = %v, want %v", got, c.want)
			}
		})
	}
}
