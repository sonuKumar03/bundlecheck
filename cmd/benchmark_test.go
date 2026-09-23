package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBenchmarkCommand(t *testing.T) {
	rawLog := `
goos: darwin
goarch: arm64
pkg: bundleradar/internal/advisor
cpu: Apple M4
BenchmarkAdvisorScaling-10    	    1372	    889770 ns/op	 1191148 B/op	    9416 allocs/op
pkg: bundleradar/internal/angular
BenchmarkParse-10    	   83103	     12445 ns/op	    5416 B/op	      46 allocs/op
`
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "benchmark.txt")
	if err := os.WriteFile(logFile, []byte(rawLog), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// 1. Text mode
	var out, errOut bytes.Buffer
	code := Execute([]string{"benchmark", logFile}, &out, &errOut)
	if code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	textOut := out.String()
	if !strings.Contains(textOut, "Benchmark Performance (Human-Readable)") {
		t.Errorf("expected header, got: %s", textOut)
	}
	if !strings.Contains(textOut, "889.77 µs") || !strings.Contains(textOut, "12.45 µs") {
		t.Errorf("expected formatted times in text, got: %s", textOut)
	}

	// 2. Markdown mode
	out.Reset()
	errOut.Reset()
	code = Execute([]string{"benchmark", logFile, "-f", "markdown"}, &out, &errOut)
	if code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	mdOut := out.String()
	if !strings.Contains(mdOut, "## ⚡ Benchmark Performance (Human-Readable)") {
		t.Errorf("expected markdown header, got: %s", mdOut)
	}
	if !strings.Contains(mdOut, "| **`BenchmarkParse`** |") {
		t.Errorf("expected BenchmarkParse row, got: %s", mdOut)
	}

	// 3. JSON mode
	out.Reset()
	errOut.Reset()
	code = Execute([]string{"benchmark", logFile, "-f", "json"}, &out, &errOut)
	if code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	jsonOut := out.String()
	if !strings.Contains(jsonOut, `"TimePerOp": "12.45 µs"`) {
		t.Errorf("expected JSON TimePerOp, got: %s", jsonOut)
	}
}
