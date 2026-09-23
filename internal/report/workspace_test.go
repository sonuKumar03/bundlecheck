package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/analysis"
	"github.com/sonuKumar03/bundleradar/internal/snapshot"
	"github.com/sonuKumar03/bundleradar/internal/workspace"
)

func TestWorkspaceHumanReport(t *testing.T) {
	r := workspace.NewResult("/workspace", "build", "production")
	r.Apps = []workspace.App{{Name: "admin", Status: "analyzed", Stats: "/workspace/dist/admin/stats.json", Dist: "/workspace/dist/admin/browser", Analysis: &analysis.AnalysisResult{Summary: snapshot.Totals{InitialJS: 1024, TotalJS: 1024}, Packages: []snapshot.Package{{Name: "exceljs", InitialBytes: 512, TotalBytes: 512}}}}, {Name: "portal", Status: "analyzed", Stats: "/workspace/dist/portal/stats.json", Dist: "/workspace/dist/portal/browser", Analysis: &analysis.AnalysisResult{Summary: snapshot.Totals{InitialJS: 2048, LazyJS: 128, TotalJS: 2176}}}, {Name: "missing", Status: "missing-artifacts", Diagnostic: "no stats"}}
	r.Complete = false
	r.Packages = []workspace.Contributor{{Name: "exceljs", Bytes: workspace.Bytes{InitialBytes: 2048, TotalBytes: 2048}, Apps: map[string]*workspace.Bytes{"admin": {InitialBytes: 512, TotalBytes: 512}, "portal": {InitialBytes: 1536, TotalBytes: 1536}, "missing": nil}}, {Name: "@angular/core", Bytes: workspace.Bytes{InitialBytes: 200, TotalBytes: 200}, Apps: map[string]*workspace.Bytes{"admin": {InitialBytes: 100, TotalBytes: 100}, "portal": {InitialBytes: 100, TotalBytes: 100}, "missing": nil}}}
	r.Libraries = []workspace.Contributor{{Name: "reports", Bytes: workspace.Bytes{InitialBytes: 293, TotalBytes: 293}, Apps: map[string]*workspace.Bytes{"portal": {InitialBytes: 293, TotalBytes: 293}, "admin": {}, "missing": nil}}}
	before, _ := json.Marshal(r)
	for _, markdown := range []bool{false, true} {
		var out bytes.Buffer
		if err := Workspace(&out, r, TextOptions{Top: 10}, markdown); err != nil {
			t.Fatal(err)
		}
		s := out.String()
		previous := -1
		for _, section := range []string{"Key findings", "App overview", "Startup contributors", "Library dependency context", "Drill-down"} {
			index := strings.Index(s, section)
			if index <= previous {
				t.Fatalf("section order %s: %s", section, s)
			}
			previous = index
		}
		for _, want := range []string{"Partial", "2 KiB", "50.0%", "75.0%", "unavailable", "Own code", "excludes imported npm", "Framework/runtime", "--package exceljs", "dist/admin/stats.json"} {
			if !strings.Contains(s, want) {
				t.Fatalf("missing %q: %s", want, s)
			}
		}
		if strings.Contains(s, "<package-name>") || strings.Contains(s, "512 / 0 / 512") {
			t.Fatal("placeholder or packed byte counts remain")
		}
		findings := s[strings.Index(s, "Key findings"):strings.Index(s, "App overview")]
		if strings.Contains(findings, "@angular/core") {
			t.Fatal("framework presented as optimization target")
		}
	}
	after, _ := json.Marshal(r)
	if !bytes.Equal(before, after) {
		t.Fatal("rendering changed JSON result")
	}
}

func TestWorkspaceLazyRankingAndSafeCommands(t *testing.T) {
	r := workspace.NewResult("/workspace with 'quote'", "build", "production")
	r.Apps = []workspace.App{{Name: "app", Status: "analyzed", Stats: r.Root + "/dist/stats.json", Dist: r.Root + "/dist/browser", Analysis: &analysis.AnalysisResult{Summary: snapshot.Totals{InitialJS: 1024, LazyJS: 4096, TotalJS: 5120}}}}
	r.Packages = []workspace.Contributor{{Name: "small", Bytes: workspace.Bytes{LazyBytes: 512, TotalBytes: 512}, Apps: map[string]*workspace.Bytes{"app": {LazyBytes: 512, TotalBytes: 512}}}, {Name: "large", Bytes: workspace.Bytes{LazyBytes: 2048, TotalBytes: 2048}, Apps: map[string]*workspace.Bytes{"app": {LazyBytes: 2048, TotalBytes: 2048}}}}
	var out bytes.Buffer
	if err := Workspace(&out, r, TextOptions{Top: 1}, true); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	lazyStart, libraryStart := strings.Index(s, "Lazy contributors"), strings.Index(s, "Library dependency context")
	lazy := s[lazyStart:libraryStart]
	if !strings.Contains(lazy, "large") || strings.Contains(lazy, "small") {
		t.Fatalf("lazy ranking: %s", lazy)
	}
	if !strings.Contains(s, `cd '/workspace with '"'"'quote'"'"''`) || !strings.Contains(s, "--stats dist/stats.json") {
		t.Fatalf("commands: %s", s)
	}
	if r.Packages[0].Name != "small" {
		t.Fatal("lazy ranking changed JSON order")
	}
}
