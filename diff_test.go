package similar

import (
	"encoding/json"
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
