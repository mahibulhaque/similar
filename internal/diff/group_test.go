package diff

import (
	"reflect"
	"testing"
)

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
