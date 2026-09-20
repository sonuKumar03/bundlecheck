package build

import (
	"path/filepath"
	"testing"
)

func TestLoadNormalizesAndClassifiesBrowserBundle(t *testing.T) {
	base := filepath.Join("..", "..", "testdata", "lazy-import")
	s, err := Load(filepath.Join(base, "stats.json"), filepath.Join(base, "browser"))
	if err != nil {
		t.Fatal(err)
	}

	wantInitial := map[string]bool{
		"browser/main.js":    true,
		"browser/chunk-A.js": true,
		"browser/chunk-B.js": true,
		"browser/chunk-C.js": false,
		"browser/chunk-D.js": false,
	}
	if len(s.Outputs) != len(wantInitial) {
		t.Fatalf("got %d browser outputs, want %d", len(s.Outputs), len(wantInitial))
	}
	for _, output := range s.Outputs {
		want, ok := wantInitial[output.Path]
		if !ok {
			t.Errorf("unexpected output %q", output.Path)
			continue
		}
		if output.Initial != want {
			t.Errorf("output %q initial = %v, want %v", output.Path, output.Initial, want)
		}
	}
}
