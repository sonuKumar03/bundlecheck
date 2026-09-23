package comparison

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/analysis"
	"github.com/sonuKumar03/bundleradar/internal/snapshot"
)

func TestParse(t *testing.T) {
	valid, err := os.ReadFile("../../testdata/comparison/before.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		edit func(map[string]any)
		want string
	}{
		{"valid", nil, ""},
		{"unknown metadata", func(m map[string]any) { m["future"] = true }, ""},
		{"different tool version", func(m map[string]any) { m["toolVersion"] = "0.2.0" }, ""},
		{"missing schema", func(m map[string]any) { delete(m, "schemaVersion") }, "schemaVersion"},
		{"unsupported schema", func(m map[string]any) { m["schemaVersion"] = "2" }, "schemaVersion"},
		{"wrong command", func(m map[string]any) { m["command"] = "compare" }, "command"},
		{"missing version", func(m map[string]any) { delete(m, "toolVersion") }, "toolVersion"},
		{"blank version", func(m map[string]any) { m["toolVersion"] = " " }, "toolVersion"},
		{"missing summary", func(m map[string]any) { delete(m, "summary") }, "summary"},
		{"null summary", func(m map[string]any) { m["summary"] = nil }, "summary"},
		{"wrong summary type", func(m map[string]any) { m["summary"] = []any{} }, "JSON"},
		{"missing packages", func(m map[string]any) { delete(m, "packages") }, "packages"},
		{"null packages", func(m map[string]any) { m["packages"] = nil }, "packages"},
		{"wrong packages type", func(m map[string]any) { m["packages"] = map[string]any{} }, "JSON"},
		{"empty packages", func(m map[string]any) { m["packages"] = []any{} }, ""},
		{"missing count", func(m map[string]any) { delete(m["summary"].(map[string]any), "initialJs") }, "initialJs"},
		{"null count", func(m map[string]any) { m["summary"].(map[string]any)["lazyJs"] = nil }, "lazyJs"},
		{"negative count", func(m map[string]any) { m["summary"].(map[string]any)["initialJs"] = -1 }, "nonnegative"},
		{"fractional count", func(m map[string]any) { m["summary"].(map[string]any)["initialJs"] = 1.5 }, "JSON"},
		{"string count", func(m map[string]any) { m["summary"].(map[string]any)["initialJs"] = "1000" }, "JSON"},
		{"inconsistent summary", func(m map[string]any) { m["summary"].(map[string]any)["totalJs"] = 1501 }, "total"},
		{"overflowing summary", func(m map[string]any) {
			m["summary"] = map[string]any{"initialJs": int64(math.MaxInt64), "lazyJs": 1, "totalJs": int64(math.MaxInt64)}
		}, "total"},
		{"blank package", func(m map[string]any) { m["packages"].([]any)[0].(map[string]any)["name"] = " " }, "name"},
		{"missing package count", func(m map[string]any) { delete(m["packages"].([]any)[0].(map[string]any), "totalBytes") }, "totalBytes"},
		{"null package count", func(m map[string]any) { m["packages"].([]any)[0].(map[string]any)["lazyBytes"] = nil }, "lazyBytes"},
		{"overflowing package", func(m map[string]any) {
			m["packages"] = []any{map[string]any{"name": "large", "initialBytes": int64(math.MaxInt64), "lazyBytes": 1, "totalBytes": int64(math.MaxInt64)}}
		}, "totalBytes"},
		{"negative package", func(m map[string]any) { m["packages"].([]any)[0].(map[string]any)["lazyBytes"] = -1 }, "nonnegative"},
		{"inconsistent package", func(m map[string]any) { m["packages"].([]any)[0].(map[string]any)["totalBytes"] = 201 }, "total"},
		{"duplicate package", func(m map[string]any) { m["packages"] = append(m["packages"].([]any), m["packages"].([]any)[0]) }, "duplicate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var m map[string]any
			if err := json.Unmarshal(valid, &m); err != nil {
				t.Fatal(err)
			}
			if tt.edit != nil {
				tt.edit(m)
			}
			data, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(t.TempDir(), "snapshot.json")
			if err := os.WriteFile(file, data, 0600); err != nil {
				t.Fatal(err)
			}
			r, err := Parse(file)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("got %v, want %q", err, tt.want)
				}
				return
			}
			if err != nil || r.Summary.TotalJS != 1500 || r.Packages == nil {
				t.Fatalf("result %v, error %v", r, err)
			}
		})
	}
	for _, data := range []string{"{", "null", "[]", "{} {}", `{"summary":{"initialJs":9223372036854775808}}`} {
		file := filepath.Join(t.TempDir(), "invalid.json")
		if err := os.WriteFile(file, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Parse(file); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
	if _, err := Parse(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("accepted missing file")
	}
	maximum := filepath.Join(t.TempDir(), "maximum.json")
	if err := os.WriteFile(maximum, []byte(`{"schemaVersion":"1","toolVersion":"0.1.0","command":"summary","summary":{"initialJs":9223372036854775807,"lazyJs":0,"totalJs":9223372036854775807},"packages":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if r, err := Parse(maximum); err != nil || r.Summary.TotalJS != math.MaxInt64 {
		t.Fatalf("valid int64 maximum: %v, %v", r, err)
	}
}

func TestCompare(t *testing.T) {
	before, err := Parse("../../testdata/comparison/before.json")
	if err != nil {
		t.Fatal(err)
	}
	after, err := Parse("../../testdata/comparison/after.json")
	if err != nil {
		t.Fatal(err)
	}
	r := Compare(before, after)
	if r.SchemaVersion != "1" || r.ToolVersion != analysis.ToolVersion || r.Command != "compare" {
		t.Fatalf("metadata: %+v", r)
	}
	if r.Summary.Before != before.Summary || r.Summary.After != after.Summary || r.Summary.Delta != (snapshot.Totals{InitialJS: 100, LazyJS: 200, TotalJS: 300}) {
		t.Fatalf("totals: %+v", r.Summary)
	}
	wantNames := []string{"grow", "removed", "shrink", "moved", "added", "lazy-grow", "@angular/core"}
	wantStatus := []string{"changed", "removed", "changed", "changed", "added", "changed", "unchanged"}
	wantDelta := []Bytes{{200, 0, 200}, {-150, 0, -150}, {-150, 0, -150}, {-100, 100, 0}, {100, 0, 100}, {0, 200, 200}, {}}
	for i, p := range r.Packages {
		if p.Name != wantNames[i] || p.Status != wantStatus[i] || p.Delta != wantDelta[i] {
			t.Fatalf("package %d: %+v", i, p)
		}
	}
	if len(r.Packages) != len(wantNames) {
		t.Fatalf("packages: %+v", r.Packages)
	}
	copyBefore := append([]snapshot.Package(nil), before.Packages...)
	if !reflect.DeepEqual(r, Compare(before, after)) || !reflect.DeepEqual(copyBefore, before.Packages) {
		t.Fatal("unstable comparison or input mutation")
	}
	reversed := Compare(after, before)
	if reversed.Summary.Delta != (snapshot.Totals{InitialJS: -100, LazyJS: -200, TotalJS: -300}) {
		t.Fatalf("reverse: %+v", reversed.Summary)
	}
	identical := Compare(before, before)
	if identical.Summary.Delta != (snapshot.Totals{}) {
		t.Fatal("identical summary changed")
	}
	for _, p := range identical.Packages {
		if p.Status != "unchanged" || p.Delta != (Bytes{}) {
			t.Fatal("identical package changed")
		}
	}
}

func TestCompareEmptyAndLimits(t *testing.T) {
	zero := &analysis.AnalysisResult{Packages: []snapshot.Package{}}
	maximum := &analysis.AnalysisResult{Summary: snapshot.Totals{InitialJS: math.MaxInt64, TotalJS: math.MaxInt64}, Packages: []snapshot.Package{{Name: "large", InitialBytes: math.MaxInt64, TotalBytes: math.MaxInt64}}}
	if r := Compare(zero, zero); r.Packages == nil || len(r.Packages) != 0 {
		t.Fatal("empty packages must be []")
	}
	for _, tt := range []struct {
		before, after *analysis.AnalysisResult
		delta         int64
	}{{zero, maximum, math.MaxInt64}, {maximum, zero, -math.MaxInt64}} {
		r := Compare(tt.before, tt.after)
		if r.Summary.Delta.InitialJS != tt.delta || r.Packages[0].Delta.InitialBytes != tt.delta {
			t.Fatalf("int64 boundary: %+v", r)
		}
	}
}

func TestCompare_Sources(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary:  snapshot.Totals{InitialJS: 5000, TotalJS: 5000},
		Packages: []snapshot.Package{},
		Sources: []analysis.SourceContribution{
			{Name: "src/app/shared", InitialBytes: 1000, LazyBytes: 0, TotalBytes: 1000},
			{Name: "src/app/removed-dir", InitialBytes: 500, LazyBytes: 0, TotalBytes: 500},
			{Name: "src/app/unchanged-dir", InitialBytes: 200, LazyBytes: 0, TotalBytes: 200},
		},
	}
	after := &analysis.AnalysisResult{
		Summary:  snapshot.Totals{InitialJS: 6500, TotalJS: 6500},
		Packages: []snapshot.Package{},
		Sources: []analysis.SourceContribution{
			{Name: "src/app/shared", InitialBytes: 2500, LazyBytes: 0, TotalBytes: 2500},
			{Name: "src/app/added-dir", InitialBytes: 500, LazyBytes: 0, TotalBytes: 500},
			{Name: "src/app/unchanged-dir", InitialBytes: 200, LazyBytes: 0, TotalBytes: 200},
		},
	}

	r := Compare(before, after)
	if len(r.Sources) != 4 {
		t.Fatalf("expected 4 source changes, got %d: %+v", len(r.Sources), r.Sources)
	}

	// Verify order: shared (delta 1500), added-dir (delta 500), removed-dir (delta -500), unchanged-dir (delta 0)
	if r.Sources[0].Name != "src/app/shared" || r.Sources[0].Status != "changed" || r.Sources[0].Delta.InitialBytes != 1500 {
		t.Errorf("expected shared changed +1500, got %+v", r.Sources[0])
	}
	if r.Sources[1].Name != "src/app/added-dir" || r.Sources[1].Status != "added" || r.Sources[1].Delta.InitialBytes != 500 {
		t.Errorf("expected added-dir added +500, got %+v", r.Sources[1])
	}
	if r.Sources[2].Name != "src/app/removed-dir" || r.Sources[2].Status != "removed" || r.Sources[2].Delta.InitialBytes != -500 {
		t.Errorf("expected removed-dir removed -500, got %+v", r.Sources[2])
	}
	if r.Sources[3].Name != "src/app/unchanged-dir" || r.Sources[3].Status != "unchanged" || r.Sources[3].Delta.InitialBytes != 0 {
		t.Errorf("expected unchanged-dir unchanged, got %+v", r.Sources[3])
	}
}
