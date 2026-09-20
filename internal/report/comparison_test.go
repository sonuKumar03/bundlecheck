package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/comparison"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

func TestComparisonText(t *testing.T) {
	r := &comparison.Result{Summary: comparison.SummaryChange{
		Before: snapshot.Totals{InitialJS: 1024, LazyJS: 2048, TotalJS: 3072},
		After:  snapshot.Totals{InitialJS: 2048, LazyJS: 1024, TotalJS: 3072},
		Delta:  snapshot.Totals{InitialJS: 1024, LazyJS: -1024},
	}}
	for i := 0; i < 11; i++ {
		r.Packages = append(r.Packages, comparison.PackageChange{Name: fmt.Sprintf("package-%02d", i), Status: "changed", Delta: comparison.Bytes{InitialBytes: int64(20 - i)}})
	}
	r.Packages = append(r.Packages, comparison.PackageChange{Name: "same", Status: "unchanged"})
	var out bytes.Buffer
	if err := ComparisonText(&out, r); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Before", "After", "Change", "1 KB", "2 KB", "+1 KB", "-1 KB", "0 B", "Initial", "Lazy", "Total"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
	if strings.Count(out.String(), "package-") != 10 || strings.Contains(out.String(), "package-10") || strings.Contains(out.String(), "same") {
		t.Fatalf("text limit: %s", out.String())
	}
	out.Reset()
	if err := ComparisonText(&out, &comparison.Result{Packages: []comparison.PackageChange{{Name: "lazy", Status: "added", Delta: comparison.Bytes{LazyBytes: 10, TotalBytes: 10}}}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "lazy") || !strings.Contains(out.String(), "+10 B") {
		t.Fatal("omitted lazy-only change")
	}
	out.Reset()
	if err := ComparisonText(&out, &comparison.Result{Packages: []comparison.PackageChange{{Name: "same", Status: "unchanged"}}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "(none)") {
		t.Fatal("missing no-change message")
	}
	if err := ComparisonText(failingWriter{}, r); err == nil {
		t.Fatal("lost writer error")
	}
}

func TestComparisonJSON(t *testing.T) {
	b, err := comparison.Parse("../../testdata/comparison/before.json")
	if err != nil {
		t.Fatal(err)
	}
	a, err := comparison.Parse("../../testdata/comparison/after.json")
	if err != nil {
		t.Fatal(err)
	}
	r := comparison.Compare(b, a)
	var first, second bytes.Buffer
	if err := JSON(&first, r); err != nil {
		t.Fatal(err)
	}
	if err := JSON(&second, r); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() || !json.Valid(first.Bytes()) || !strings.HasSuffix(first.String(), "\n") {
		t.Fatal("unstable or invalid JSON")
	}
	var decoded comparison.Result
	if err := json.Unmarshal(first.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Packages) != 7 || decoded.Packages[1].Status != "removed" || decoded.Packages[1].After != (comparison.Bytes{}) || decoded.Summary.Delta.TotalJS != 300 {
		t.Fatalf("JSON: %+v", decoded)
	}
	first.Reset()
	if err := JSON(&first, &comparison.Result{SchemaVersion: "1", ToolVersion: "0.1.0", Command: "compare", Packages: []comparison.PackageChange{}}); err != nil {
		t.Fatal(err)
	}
	const want = "{\n  \"schemaVersion\": \"1\",\n  \"toolVersion\": \"0.1.0\",\n  \"command\": \"compare\",\n  \"summary\": {\n    \"before\": {\n      \"initialJs\": 0,\n      \"lazyJs\": 0,\n      \"totalJs\": 0\n    },\n    \"after\": {\n      \"initialJs\": 0,\n      \"lazyJs\": 0,\n      \"totalJs\": 0\n    },\n    \"delta\": {\n      \"initialJs\": 0,\n      \"lazyJs\": 0,\n      \"totalJs\": 0\n    }\n  },\n  \"packages\": []\n}\n"
	if first.String() != want {
		t.Fatalf("empty JSON: %s", first.String())
	}
	if err := JSON(failingWriter{}, r); err == nil {
		t.Fatal("lost JSON writer error")
	}
}

func TestComparisonTextWithFindings(t *testing.T) {
	r := &comparison.Result{
		Summary: comparison.SummaryChange{
			Before: snapshot.Totals{InitialJS: 100 * 1024, TotalJS: 100 * 1024},
			After:  snapshot.Totals{InitialJS: 142 * 1024, TotalJS: 142 * 1024},
			Delta:  snapshot.Totals{InitialJS: 42 * 1024, TotalJS: 42 * 1024},
		},
		Findings: []comparison.Finding{
			{
				Name:       "chart.js",
				DeltaBytes: 31 * 1024,
				Kind:       "package",
				Chunks:     []string{"dist/browser/main.js"},
				TracePath:  []string{"src/main.ts", "node_modules/chart.js/auto.js"},
			},
			{
				Name:       "(unattributed)",
				DeltaBytes: 11 * 1024,
				Kind:       "unattributed",
				Reason:     "Growth in application sources",
			},
		},
	}

	var out bytes.Buffer
	if err := ComparisonText(&out, r); err != nil {
		t.Fatal(err)
	}

	text := out.String()
	for _, want := range []string{
		"Regression explanation",
		"+31 KB",
		"chart.js -> dist/browser/main.js",
		"src/main.ts",
		"-> node_modules/chart.js/auto.js",
		"+11 KB",
		"(unattributed) (Growth in application sources)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in output:\n%s", want, text)
		}
	}
}
