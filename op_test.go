package similar

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDiffOpJSONRoundTrip(t *testing.T) {
	ops := []DiffOp{
		{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 3, NewLen: 3},
		{Tag: Delete, OldIndex: 3, NewIndex: 3, OldLen: 2},
		{Tag: Insert, OldIndex: 5, NewIndex: 3, NewLen: 4},
		{Tag: Replace, OldIndex: 5, NewIndex: 7, OldLen: 1, NewLen: 2},
	}
	for _, op := range ops {
		data, err := json.Marshal(op)
		if err != nil {
			t.Fatalf("marshal %v: %v", op, err)
		}
		var got DiffOp
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal %s: %v", data, err)
		}
		if got != op {
			t.Fatalf("round trip: got %v, want %v (json=%s)", got, op, data)
		}
	}
}

func TestDiffTagJSONStableNames(t *testing.T) {
	data, err := json.Marshal(DiffOp{Tag: Replace, OldIndex: 1, NewIndex: 2, OldLen: 3, NewLen: 4})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"tag":"replace","old_index":1,"new_index":2,"old_len":3,"new_len":4}`
	if string(data) != want {
		t.Fatalf("json = %s, want %s", data, want)
	}
}

func TestDiffOpRangesAndLen(t *testing.T) {
	eq := DiffOp{Tag: Equal, OldIndex: 2, NewIndex: 5, OldLen: 3, NewLen: 3}
	if os, oe := eq.OldRange(); os != 2 || oe != 5 {
		t.Fatalf("OldRange = (%d,%d), want (2,5)", os, oe)
	}
	if ns, ne := eq.NewRange(); ns != 5 || ne != 8 {
		t.Fatalf("NewRange = (%d,%d), want (5,8)", ns, ne)
	}
	if eq.Len() != 3 {
		t.Fatalf("Len = %d, want 3", eq.Len())
	}
	del := DiffOp{Tag: Delete, OldIndex: 0, NewIndex: 0, OldLen: 2}
	if _, ne := del.NewRange(); ne != 0 {
		t.Fatalf("delete new range end = %d, want 0", ne)
	}
}

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

func TestGroupDiffOpsEmpty(t *testing.T) {
	if got := GroupDiffOps(nil, 3); got != nil {
		t.Fatalf("GroupDiffOps(nil) = %v, want nil", got)
	}
}

func TestGroupDiffOpsTrimsContextSingleGroup(t *testing.T) {
	ops := []DiffOp{
		{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 10, NewLen: 10},
		{Tag: Replace, OldIndex: 10, NewIndex: 10, OldLen: 2, NewLen: 2},
		{Tag: Equal, OldIndex: 12, NewIndex: 12, OldLen: 10, NewLen: 10},
	}
	got := GroupDiffOps(ops, 3)
	want := [][]DiffOp{{
		{Tag: Equal, OldIndex: 7, NewIndex: 7, OldLen: 3, NewLen: 3},
		{Tag: Replace, OldIndex: 10, NewIndex: 10, OldLen: 2, NewLen: 2},
		{Tag: Equal, OldIndex: 12, NewIndex: 12, OldLen: 3, NewLen: 3},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groups = %v, want %v", got, want)
	}
}

func TestGroupDiffOpsSplitsLongEqualRun(t *testing.T) {
	ops := []DiffOp{
		{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 2, NewLen: 2},
		{Tag: Delete, OldIndex: 2, NewIndex: 2, OldLen: 1},
		{Tag: Equal, OldIndex: 3, NewIndex: 2, OldLen: 20, NewLen: 20},
		{Tag: Insert, OldIndex: 23, NewIndex: 22, NewLen: 1},
		{Tag: Equal, OldIndex: 24, NewIndex: 23, OldLen: 2, NewLen: 2},
	}
	got := GroupDiffOps(ops, 3)
	want := [][]DiffOp{
		{
			{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 2, NewLen: 2},
			{Tag: Delete, OldIndex: 2, NewIndex: 2, OldLen: 1},
			{Tag: Equal, OldIndex: 3, NewIndex: 2, OldLen: 3, NewLen: 3},
		},
		{
			{Tag: Equal, OldIndex: 20, NewIndex: 19, OldLen: 3, NewLen: 3},
			{Tag: Insert, OldIndex: 23, NewIndex: 22, NewLen: 1},
			{Tag: Equal, OldIndex: 24, NewIndex: 23, OldLen: 2, NewLen: 2},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groups = %#v\nwant %#v", got, want)
	}
}

func TestGroupDiffOpsDropsLoneEqual(t *testing.T) {
	ops := []DiffOp{
		{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 5, NewLen: 5},
	}
	if got := GroupDiffOps(ops, 3); len(got) != 0 {
		t.Fatalf("groups = %v, want empty (lone equal dropped)", got)
	}
}

func TestGroupDiffOpsDoesNotMutateInput(t *testing.T) {
	ops := []DiffOp{
		{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 10, NewLen: 10},
		{Tag: Delete, OldIndex: 10, NewIndex: 10, OldLen: 1},
	}
	before := make([]DiffOp, len(ops))
	copy(before, ops)
	_ = GroupDiffOps(ops, 3)
	if !reflect.DeepEqual(ops, before) {
		t.Fatalf("input mutated: %v, want %v", ops, before)
	}
}
