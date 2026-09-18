package angular

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	for _, tt := range []struct{ name, data, wantError string }{
		{"valid", `{"inputs":{"src/main.ts":{"bytes":100}},"outputs":{"main.js":{"bytes":50,"inputs":{"src/main.ts":{"bytesInOutput":40}}}},"future":true}`, ""},
		{"malformed", `{"inputs":`, "JSON"},
		{"missing structure", `{}`, "inputs"},
		{"null inputs", `{"inputs":null,"outputs":{}}`, "inputs"},
		{"array outputs", `{"inputs":{},"outputs":[]}`, "outputs"},
		{"missing outputs", `{"inputs":{}}`, "outputs"},
		{"negative", `{"inputs":{},"outputs":{"main.js":{"bytes":-1,"inputs":{}}}}`, "bytes"},
		{"missing bytes", `{"inputs":{"a":{}},"outputs":{}}`, "bytes"},
		{"null bytes", `{"inputs":{"a":{"bytes":null}},"outputs":{}}`, "bytes"},
		{"fractional", `{"inputs":{},"outputs":{"main.js":{"bytes":1.5,"inputs":{}}}}`, "JSON"},
		{"missing contribution", `{"inputs":{"a":{"bytes":1}},"outputs":{"main.js":{"bytes":1,"inputs":{"a":{}}}}}`, "bytesInOutput"},
		{"unknown input", `{"inputs":{},"outputs":{"main.js":{"bytes":1,"inputs":{"a":{"bytesInOutput":1}}}}}`, "input"},
		{"trailing document", `{"inputs":{},"outputs":{}} {}`, "JSON"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "stats.json")
			if err := os.WriteFile(p, []byte(tt.data), 0600); err != nil {
				t.Fatal(err)
			}
			m, err := Parse(p)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("got %v, want error containing %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if m.Outputs["main.js"].Inputs["src/main.ts"].BytesInOutput != 40 {
				t.Fatal("lost contribution")
			}
		})
	}
	if _, err := Parse(filepath.Join(t.TempDir(), "missing.json")); err == nil || !strings.Contains(err.Error(), "stats") {
		t.Fatalf("missing file: %v", err)
	}
}

func TestNormalize(t *testing.T) {
	m := &Metafile{
		Inputs: map[string]Input{"src/z.ts": {Bytes: 100}, `node_modules\pdfjs-dist\a.js`: {Bytes: 200}},
		Outputs: map[string]Output{
			"./main.js": {Bytes: 80, EntryPoint: `./src\z.ts`, Inputs: map[string]OutputInput{
				`node_modules\pdfjs-dist\a.js`: {BytesInOutput: 20}, "node_modules/pdfjs-dist/a.js": {BytesInOutput: 20},
			}, Imports: []Import{{Path: "./lazy.js", Kind: "dynamic-import"}, {Path: "https://example.com/a.js", Kind: "import-statement", External: true}}},
			"lazy.js": {Bytes: 30},
		},
	}
	s, err := Normalize(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Outputs) != 2 || s.Outputs[0].Path != "lazy.js" || s.Outputs[1].Path != "main.js" {
		t.Fatalf("outputs: %+v", s.Outputs)
	}
	o := s.Outputs[1]
	if o.EntryPoint != "src/z.ts" {
		t.Fatalf("entryPoint: %q", o.EntryPoint)
	}
	if len(o.Inputs) != 1 || o.Inputs[0].Bytes != 20 || o.Inputs[0].Input != "node_modules/pdfjs-dist/a.js" {
		t.Fatalf("contributions: %+v", o.Inputs)
	}
	flags := make(map[string]bool)
	for _, imp := range o.Imports {
		flags[imp.Path] = imp.Dynamic || imp.External
	}
	if !flags["lazy.js"] || !flags["https://example.com/a.js"] {
		t.Fatalf("imports: %+v", o.Imports)
	}
	imports := m.Outputs["./main.js"].Imports
	imports[0], imports[1] = imports[1], imports[0]
	s2, err := Normalize(m)
	if err != nil || !reflect.DeepEqual(s, s2) {
		t.Fatal("unstable normalization")
	}
	m.Outputs["./main.js"].Inputs["node_modules/pdfjs-dist/a.js"] = OutputInput{BytesInOutput: 21}
	if _, err := Normalize(m); err == nil {
		t.Fatal("accepted conflicting contributions")
	}
	if _, err := Normalize(nil); err == nil {
		t.Fatal("accepted nil metafile")
	}
}

func TestNormalizeAssetImport(t *testing.T) {
	s, err := Normalize(&Metafile{Outputs: map[string]Output{"main.js": {Imports: []Import{{Path: "worker.js", Kind: "file-loader"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if !s.Outputs[0].Imports[0].Asset {
		t.Fatal("file-loader references must not become static JS imports")
	}
}

func BenchmarkParse(b *testing.B) {
	fixturePath := filepath.Join("..", "..", "testdata", "minimal", "stats.json")
	if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
		b.Skip("testdata fixture not present")
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Parse(fixturePath)
		if err != nil {
			b.Fatal(err)
		}
	}
}

