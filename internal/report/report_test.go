package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

func TestText(t *testing.T) {
	for _, tt := range []struct {
		name   string
		totals snapshot.Totals
		want   []string
	}{
		{"zero", snapshot.Totals{}, []string{"Initial JS", "0 B"}},
		{"binary units", snapshot.Totals{InitialJS: 933888, LazyJS: 3575644, TotalJS: 4509532}, []string{"912 KB", "3.41 MB", "4.3 MB"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Text(&out, &analysis.AnalysisResult{Summary: tt.totals, Packages: []snapshot.Package{}}); err != nil {
				t.Fatal(err)
			}
			for _, want := range tt.want {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("missing %q in %q", want, out.String())
				}
			}
		})
	}
	r := &analysis.AnalysisResult{}
	for i := 0; i < 11; i++ {
		r.Packages = append(r.Packages, snapshot.Package{Name: fmt.Sprintf("package-%02d", i), InitialBytes: int64(110 - i)})
	}
	r.Packages = append(r.Packages, snapshot.Package{Name: "lazy-only", LazyBytes: 100})
	var out bytes.Buffer
	if err := Text(&out, r); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "package-") != 10 || strings.Contains(out.String(), "package-10") || strings.Contains(out.String(), "lazy-only") {
		t.Fatalf("text limit: %s", out.String())
	}
}

func TestJSON(t *testing.T) {
	r := &analysis.AnalysisResult{SchemaVersion: "1", ToolVersion: "0.1.0", Command: "summary", Summary: snapshot.Totals{InitialJS: 1024, TotalJS: 1024}, Packages: []snapshot.Package{}}
	var first, second bytes.Buffer
	if err := JSON(&first, r); err != nil {
		t.Fatal(err)
	}
	if err := JSON(&second, r); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() || !json.Valid(first.Bytes()) {
		t.Fatalf("invalid or unstable JSON: %s", first.String())
	}
	const want = "{\n  \"schemaVersion\": \"1\",\n  \"toolVersion\": \"0.1.0\",\n  \"command\": \"summary\",\n  \"summary\": {\n    \"initialJs\": 1024,\n    \"lazyJs\": 0,\n    \"totalJs\": 1024\n  },\n  \"packages\": []\n}\n"
	if first.String() != want {
		t.Fatalf("got %s, want %s", first.String(), want)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("write failed") }

func TestWriteErrors(t *testing.T) {
	r := &analysis.AnalysisResult{}
	for name, fn := range map[string]func() error{"text": func() error { return Text(failingWriter{}, r) }, "json": func() error { return JSON(failingWriter{}, r) }} {
		t.Run(name, func(t *testing.T) {
			if fn() == nil {
				t.Fatal("lost writer error")
			}
		})
	}
}
