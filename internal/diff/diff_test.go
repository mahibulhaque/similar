package diff

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

func TestNoopHookReturnsNil(t *testing.T) {
	var h NoopHook
	if err := h.Equal(0, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err := h.Delete(0, 1, 0); err != nil {
		t.Fatal(err)
	}
	if err := h.Insert(0, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err := h.Replace(0, 1, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err := h.Finish(); err != nil {
		t.Fatal(err)
	}
}

// overrideOnly embeds NoopHook and overrides a single callback, proving the
// selective-override pattern compiles and works.
type overrideOnly struct {
	NoopHook
	equals int
}

func (o *overrideOnly) Equal(int, int, int) error { o.equals++; return nil }

func TestNoopHookSelectiveOverride(t *testing.T) {
	o := &overrideOnly{}
	_ = o.Equal(0, 0, 1)
	_ = o.Delete(0, 1, 0) // inherited no-op
	if o.equals != 1 {
		t.Fatalf("equals = %d, want 1", o.equals)
	}
}

func TestCaptureAccumulates(t *testing.T) {
	c := NewCapture()
	_ = c.Equal(0, 0, 2)
	_ = c.Delete(2, 1, 2)
	_ = c.Insert(3, 2, 3)
	ops := c.Ops()
	if len(ops) != 3 {
		t.Fatalf("len = %d, want 3", len(ops))
	}
	if ops[0].Tag != Equal || ops[1].Tag != Delete || ops[2].Tag != Insert {
		t.Fatalf("unexpected tags: %v", ops)
	}
}

func TestReplaceHookCoalesces(t *testing.T) {
	c := NewCapture()
	r := NewReplace(c)
	// equal, then delete+insert adjacent (should coalesce into replace).
	_ = r.Equal(0, 0, 3)
	_ = r.Delete(3, 2, 3)
	_ = r.Insert(5, 3, 3)
	_ = r.Finish()

	ops := c.Ops()
	want := []DiffOp{
		{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 3, NewLen: 3},
		{Tag: Replace, OldIndex: 3, NewIndex: 3, OldLen: 2, NewLen: 3},
	}
	if len(ops) != len(want) {
		t.Fatalf("ops = %v, want %v", ops, want)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Fatalf("op %d = %v, want %v", i, ops[i], want[i])
		}
	}
}

func TestReplaceHookLoneDeleteStaysDelete(t *testing.T) {
	c := NewCapture()
	r := NewReplace(c)
	_ = r.Delete(0, 2, 0)
	_ = r.Equal(2, 0, 1)
	_ = r.Finish()
	ops := c.Ops()
	if len(ops) != 2 || ops[0].Tag != Delete || ops[1].Tag != Equal {
		t.Fatalf("ops = %v, want [Delete Equal]", ops)
	}
}

func TestCompactMergesAndReplays(t *testing.T) {
	// Compact + Replace should still reconstruct new and stay minimal-cost.
	old := []int{1, 2, 3, 4, 5, 6}
	new := []int{1, 2, 9, 4, 5, 6}
	c := NewCapture()
	comp := NewCompact[int](NewReplace(c), old, new)
	// Feed a raw equal/delete/insert script (as the core would).
	_ = comp.Equal(0, 0, 2)
	_ = comp.Delete(2, 1, 2)
	_ = comp.Insert(3, 2, 1)
	_ = comp.Equal(3, 3, 3)
	_ = comp.Finish()

	out := make([]int, 0, len(new))
	for _, op := range c.Ops() {
		switch op.Tag {
		case Equal:
			out = append(out, old[op.OldIndex:op.OldIndex+op.OldLen]...)
		case Insert, Replace:
			out = append(out, new[op.NewIndex:op.NewIndex+op.NewLen]...)
		}
	}
	if len(out) != len(new) {
		t.Fatalf("reconstruct len = %d, want %d (ops=%v)", len(out), len(new), c.Ops())
	}
	for i := range new {
		if out[i] != new[i] {
			t.Fatalf("reconstruct = %v, want %v", out, new)
		}
	}
}
