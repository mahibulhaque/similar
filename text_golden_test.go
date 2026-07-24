package similar_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mahibulhaque/similar"
)

// TestTextGolden snapshots the expanded change lists of representative text
// diffs. Run with -update to regenerate the fixtures.
func TestTextGolden(t *testing.T) {
	fixtures := []struct {
		name     string
		old, new string
		changes  func(old, new string) []similar.Change
	}{
		{
			name: "lines_replace",
			old:  "Hello World\nsome stuff here\nsome more stuff here\n",
			new:  "Hello World\nsome amazing stuff here\nsome more stuff here\n",
			changes: func(old, new string) []similar.Change {
				return slices.Collect(similar.DiffLines(old, new).AllChanges())
			},
		},
		{
			name: "words_replace",
			old:  "the quick brown fox",
			new:  "the slow brown cat",
			changes: func(old, new string) []similar.Change {
				return slices.Collect(similar.DiffWords(old, new).AllChanges())
			},
		},
		{
			name: "chars_replace",
			old:  "abcdef",
			new:  "abcDDf",
			changes: func(old, new string) []similar.Change {
				return slices.Collect(similar.DiffChars(old, new).AllChanges())
			},
		},
	}
	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			got, err := json.MarshalIndent(fx.changes(fx.old, fx.new), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			path := filepath.Join("testdata", "text_"+fx.name+".golden")
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update): %v", err)
			}
			if string(got) != string(want) {
				t.Fatalf("golden mismatch for %s:\ngot:\n%s\nwant:\n%s", fx.name, got, want)
			}
		})
	}
}
