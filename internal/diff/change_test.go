package diff

import (
	"encoding/json"
	"testing"
)

func TestChangeTagString(t *testing.T) {
	cases := []struct {
		tag  ChangeTag
		want string
	}{
		{ChangeEqual, " "},
		{ChangeDelete, "-"},
		{ChangeInsert, "+"},
	}
	for _, c := range cases {
		if got := c.tag.String(); got != c.want {
			t.Errorf("ChangeTag(%d).String() = %q, want %q", c.tag, got, c.want)
		}
	}
}

func TestChangeTagJSONNames(t *testing.T) {
	cases := []struct {
		tag  ChangeTag
		want string
	}{
		{ChangeEqual, `"equal"`},
		{ChangeDelete, `"delete"`},
		{ChangeInsert, `"insert"`},
	}
	for _, c := range cases {
		data, err := json.Marshal(c.tag)
		if err != nil {
			t.Fatalf("marshal %v: %v", c.tag, err)
		}
		if string(data) != c.want {
			t.Errorf("marshal ChangeTag(%d) = %s, want %s", c.tag, data, c.want)
		}
		var back ChangeTag
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatalf("unmarshal %s: %v", data, err)
		}
		if back != c.tag {
			t.Errorf("round trip = %v, want %v", back, c.tag)
		}
	}
}

func TestChangeIndicesCommaOk(t *testing.T) {
	// Equal: both present.
	eq := EqualChange("a", 2, 5)
	if i, ok := eq.OldIndex(); !ok || i != 2 {
		t.Errorf("equal OldIndex = (%d,%v), want (2,true)", i, ok)
	}
	if j, ok := eq.NewIndex(); !ok || j != 5 {
		t.Errorf("equal NewIndex = (%d,%v), want (5,true)", j, ok)
	}

	// Delete: old present, new absent.
	del := DeleteChange("b", 3)
	if i, ok := del.OldIndex(); !ok || i != 3 {
		t.Errorf("delete OldIndex = (%d,%v), want (3,true)", i, ok)
	}
	if _, ok := del.NewIndex(); ok {
		t.Errorf("delete NewIndex present, want absent")
	}

	// Insert: new present, old absent.
	ins := InsertChange("c", 4)
	if _, ok := ins.OldIndex(); ok {
		t.Errorf("insert OldIndex present, want absent")
	}
	if j, ok := ins.NewIndex(); !ok || j != 4 {
		t.Errorf("insert NewIndex = (%d,%v), want (4,true)", j, ok)
	}
}

// A zero index is a real index, not an absent one: the first token of either
// sequence must still report present.
func TestChangeZeroIndexIsPresent(t *testing.T) {
	if i, ok := DeleteChange("b", 0).OldIndex(); !ok || i != 0 {
		t.Errorf("delete OldIndex = (%d,%v), want (0,true)", i, ok)
	}
	if j, ok := InsertChange("c", 0).NewIndex(); !ok || j != 0 {
		t.Errorf("insert NewIndex = (%d,%v), want (0,true)", j, ok)
	}
}

func TestChangeMissingNewlineAndString(t *testing.T) {
	cases := []struct {
		value   string
		missing bool
		str     string
	}{
		{"foo", true, "foo\n"},
		{"foo\n", false, "foo\n"},
		{"foo\r", false, "foo\r"},
		{"", true, "\n"},
	}
	for _, c := range cases {
		ch := EqualChange(c.value, 0, 0)
		if got := ch.MissingNewline(); got != c.missing {
			t.Errorf("MissingNewline(%q) = %v, want %v", c.value, got, c.missing)
		}
		if got := ch.String(); got != c.str {
			t.Errorf("String(%q) = %q, want %q", c.value, got, c.str)
		}
	}
}

func TestChangeMarshalJSONNullIndices(t *testing.T) {
	del := DeleteChange("bar", 3)
	data, err := json.Marshal(del)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"tag":"delete","old_index":3,"new_index":null,"value":"bar"}`
	if string(data) != want {
		t.Errorf("marshal delete = %s, want %s", data, want)
	}

	ins := InsertChange("baz", 4)
	data, err = json.Marshal(ins)
	if err != nil {
		t.Fatal(err)
	}
	want = `{"tag":"insert","old_index":null,"new_index":4,"value":"baz"}`
	if string(data) != want {
		t.Errorf("marshal insert = %s, want %s", data, want)
	}
}
