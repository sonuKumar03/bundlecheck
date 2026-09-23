package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyze_WithAppTargets_NoNxJson(t *testing.T) {
	tmp := t.TempDir()
	// Deliberately NO nx.json anywhere in tmp!
	app1Dir := filepath.Join(tmp, "dist", "portal")
	app2Dir := filepath.Join(tmp, "dist", "admin")
	if err := os.MkdirAll(filepath.Join(app1Dir, "browser"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(app2Dir, "browser"), 0755); err != nil {
		t.Fatal(err)
	}

	minimalStats, err := os.ReadFile("../../testdata/minimal/stats.json")
	if err != nil {
		// Try relative to workspace package
		minimalStats, err = os.ReadFile(filepath.Join("..", "..", "testdata", "minimal", "stats.json"))
		if err != nil {
			t.Fatal(err)
		}
	}
	indexHTML, err := os.ReadFile(filepath.Join("..", "..", "testdata", "minimal", "browser", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	mainJS, err := os.ReadFile(filepath.Join("..", "..", "testdata", "minimal", "browser", "main.js"))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(app1Dir, "stats.json"), minimalStats, 0644)
	_ = os.WriteFile(filepath.Join(app2Dir, "stats.json"), minimalStats, 0644)
	_ = os.WriteFile(filepath.Join(app1Dir, "browser", "index.html"), indexHTML, 0644)
	_ = os.WriteFile(filepath.Join(app2Dir, "browser", "index.html"), indexHTML, 0644)
	_ = os.WriteFile(filepath.Join(app1Dir, "browser", "main.js"), mainJS, 0644)
	_ = os.WriteFile(filepath.Join(app2Dir, "browser", "main.js"), mainJS, 0644)

	targets := []AppTarget{
		{Name: "portal", Stats: filepath.Join(app1Dir, "stats.json"), Dist: filepath.Join(app1Dir, "browser")},
		{Name: "admin", Stats: filepath.Join(app2Dir, "stats.json"), Dist: filepath.Join(app2Dir, "browser")},
	}

	res, err := Analyze(context.Background(), AnalyzeOptions{
		Root:       tmp,
		AppTargets: targets,
	})
	if err != nil {
		t.Fatalf("Analyze with explicit targets failed without nx.json: %v", err)
	}
	if !res.Complete {
		t.Fatalf("expected complete report, got incomplete: %+v", res)
	}
	if len(res.Apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(res.Apps))
	}
	if len(res.Packages) == 0 {
		t.Fatalf("expected packages to be aggregated in matrix")
	}
}
