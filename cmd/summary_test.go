package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/snapshot"
)

func TestSummaryFixtures(t *testing.T) {
	for _, tt := range []struct {
		name     string
		totals   snapshot.Totals
		packages []snapshot.Package
	}{
		{"minimal", snapshot.Totals{InitialJS: 1024, TotalJS: 1024}, []snapshot.Package{{Name: "lodash", InitialBytes: 128, TotalBytes: 128}}},
		{"lazy-import", snapshot.Totals{InitialJS: 163840, LazyJS: 307200, TotalJS: 471040}, []snapshot.Package{
			{Name: "pdfjs-dist", InitialBytes: 51200, LazyBytes: 184320, TotalBytes: 235520},
			{Name: "@angular/core", InitialBytes: 30720, TotalBytes: 30720},
			{Name: "rxjs", InitialBytes: 25600, TotalBytes: 25600},
			{Name: "date-fns", LazyBytes: 92160, TotalBytes: 92160},
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			base := filepath.Join("..", "testdata", tt.name)
			args := []string{"summary", "--stats", filepath.Join(base, "stats.json"), "--dist", filepath.Join(base, "browser"), "--format", "json"}
			var previous string
			for i := 0; i < 5; i++ {
				var out, errOut bytes.Buffer
				if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
					t.Fatalf("exit %d: %s", code, errOut.String())
				}
				var got analysis.AnalysisResult
				if err := json.Unmarshal(out.Bytes(), &got); err != nil {
					t.Fatalf("stdout not clean JSON: %s", out.String())
				}
				if got.Summary != tt.totals || !reflect.DeepEqual(got.Packages, tt.packages) {
					t.Fatalf("got %+v, want %+v, %+v", got, tt.totals, tt.packages)
				}
				if i > 0 && previous != out.String() {
					t.Fatal("nondeterministic JSON")
				}
				previous = out.String()
			}
			var out, errOut bytes.Buffer
			if Execute(args[:len(args)-2], &out, &errOut) != 0 || !strings.HasPrefix(out.String(), "Angular Bundle Summary\n") {
				t.Fatalf("default text: %s, %s", out.String(), errOut.String())
			}
		})
	}
}

func TestSummaryFileOutputAndOptions(t *testing.T) {
	base := filepath.Join("..", "testdata", "lazy-import")
	outPath := filepath.Join(t.TempDir(), "summary.json")

	args := []string{"summary", "-s", filepath.Join(base, "stats.json"), "-d", filepath.Join(base, "browser"), "-o", outPath, "-f", "json"}
	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("expected stdout empty when -o used, got %s", out.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var res analysis.AnalysisResult
	if err := json.Unmarshal(data, &res); err != nil {
		t.Fatal(err)
	}
	if res.Summary.InitialJS != 163840 {
		t.Errorf("unexpected InitialJS %d", res.Summary.InitialJS)
	}

	// Test text options: filter and top
	out.Reset()
	errOut.Reset()
	textArgs := []string{"summary", "-s", filepath.Join(base, "stats.json"), "-d", filepath.Join(base, "browser"), "--filter", "angular", "--top", "5"}
	if code := Execute(textArgs, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "@angular/core") || strings.Contains(out.String(), "pdfjs-dist") {
		t.Errorf("filter mismatch in text output: %s", out.String())
	}
}

func TestSummaryErrors(t *testing.T) {
	base := filepath.Join("..", "testdata", "minimal")
	valid := []string{"summary", "--stats", filepath.Join(base, "stats.json"), "--dist", filepath.Join(base, "browser")}
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"missing flags with no artifacts", []string{"summary"}, "no Angular build artifacts found"},
		{"missing dist", []string{"summary", "--stats", "stats.json"}, "missing --dist"},
		{"invalid format", append(append([]string{}, valid...), "--format", "yaml"), "format"},
		{"missing stats", []string{"summary", "--stats", "missing.json", "--dist", filepath.Join(base, "browser"), "--format", "json"}, "stats"},
		{"missing index", []string{"summary", "--stats", filepath.Join(base, "stats.json"), "--dist", t.TempDir(), "--format", "json"}, "index.html"},
		{"positional args", append(append([]string{}, valid...), "extra"), "unknown command"},
		{"unknown command", []string{"why"}, "unknown command"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if Execute(tt.args, &out, &errOut) == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), tt.want) {
				t.Fatalf("stdout %q, stderr %q", out.String(), errOut.String())
			}
		})
	}
	p := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(p, []byte(`{"inputs":`), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if Execute([]string{"summary", "--stats", p, "--dist", filepath.Join(base, "browser"), "--format", "json"}, &out, &errOut) == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), "JSON") {
		t.Fatalf("malformed stats: %q, %q", out.String(), errOut.String())
	}
}
