package build

import (
	"os"
	"path/filepath"
	"strings"
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

func TestLoadWithEntry_TwoEntriesAndImportedChunk(t *testing.T) {
	dir := t.TempDir()
	browserDir := filepath.Join(dir, "browser")
	if err := os.MkdirAll(browserDir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"main.js", "chunk.js", "worker.js"} {
		if err := os.WriteFile(filepath.Join(browserDir, f), []byte("console.log()"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	statsContent := `{
  "inputs": {
    "src/main.ts": {"bytes": 1000, "imports": []},
    "src/chunk.ts": {"bytes": 500, "imports": []},
    "src/worker.ts": {"bytes": 800, "imports": []}
  },
  "outputs": {
    "browser/main.js": {
      "bytes": 1500,
      "entryPoint": "src/main.ts",
      "inputs": {"src/main.ts": {"bytesInOutput": 1000}},
      "imports": [{"path": "browser/chunk.js", "kind": "import-statement"}]
    },
    "browser/chunk.js": {
      "bytes": 500,
      "inputs": {"src/chunk.ts": {"bytesInOutput": 500}},
      "imports": []
    },
    "browser/worker.js": {
      "bytes": 800,
      "entryPoint": "src/worker.ts",
      "inputs": {"src/worker.ts": {"bytesInOutput": 800}},
      "imports": []
    }
  }
}`
	statsPath := filepath.Join(dir, "stats.json")
	if err := os.WriteFile(statsPath, []byte(statsContent), 0600); err != nil {
		t.Fatal(err)
	}

	// 1. Selecting main entry: main and its non-dynamic import (chunk) are initial,
	// worker is lazy, and none of the 3 outputs are dropped.
	sMain, err := LoadWithEntry(statsPath, browserDir, "src/main.ts")
	if err != nil {
		t.Fatalf("LoadWithEntry(main) failed: %v", err)
	}
	if len(sMain.Outputs) != 3 {
		t.Fatalf("got %d outputs, want 3", len(sMain.Outputs))
	}
	wantMainInitial := map[string]bool{
		"browser/main.js":   true,
		"browser/chunk.js":  true,
		"browser/worker.js": false,
	}
	for _, o := range sMain.Outputs {
		want, ok := wantMainInitial[o.Path]
		if !ok {
			t.Errorf("unexpected output %q", o.Path)
			continue
		}
		if o.Initial != want {
			t.Errorf("main entry: output %q initial = %v, want %v", o.Path, o.Initial, want)
		}
	}

	// 2. Selecting worker entry: worker is initial, main and chunk are lazy,
	// and neither output is dropped.
	sWorker, err := LoadWithEntry(statsPath, browserDir, "src/worker.ts")
	if err != nil {
		t.Fatalf("LoadWithEntry(worker) failed: %v", err)
	}
	if len(sWorker.Outputs) != 3 {
		t.Fatalf("got %d outputs, want 3", len(sWorker.Outputs))
	}
	wantWorkerInitial := map[string]bool{
		"browser/worker.js": true,
		"browser/main.js":   false,
		"browser/chunk.js":  false,
	}
	for _, o := range sWorker.Outputs {
		want, ok := wantWorkerInitial[o.Path]
		if !ok {
			t.Errorf("unexpected output %q", o.Path)
			continue
		}
		if o.Initial != want {
			t.Errorf("worker entry: output %q initial = %v, want %v", o.Path, o.Initial, want)
		}
	}

	// 3. Load without entry fails on dist without index.html.
	if _, err := Load(statsPath, browserDir); err == nil || !strings.Contains(err.Error(), "index.html") {
		t.Fatalf("expected index.html error from Load, got %v", err)
	}
}
