package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAppTargets_ExplicitApp(t *testing.T) {
	tmp := t.TempDir()
	stats1 := filepath.Join(tmp, "portal-stats.json")
	dist1 := filepath.Join(tmp, "portal-dist")
	if err := os.WriteFile(stats1, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dist1, 0755); err != nil {
		t.Fatal(err)
	}

	targets, err := ParseAppTargets(tmp, nil, []string{"portal=" + stats1 + ":" + dist1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Name != "portal" || targets[0].Stats != stats1 || targets[0].Dist != dist1 {
		t.Errorf("unexpected target: %+v", targets[0])
	}
}

func TestParseAppTargets_ExplicitApp_AutoDist(t *testing.T) {
	tmp := t.TempDir()
	appDir := filepath.Join(tmp, "portal")
	browserDir := filepath.Join(appDir, "browser")
	if err := os.MkdirAll(browserDir, 0755); err != nil {
		t.Fatal(err)
	}
	statsFile := filepath.Join(appDir, "stats.json")
	if err := os.WriteFile(statsFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	targets, err := ParseAppTargets(tmp, nil, []string{"portal=" + statsFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Dist != browserDir {
		t.Errorf("expected dist %q, got %q", browserDir, targets[0].Dist)
	}
}

func TestParseAppTargets_ConventionProjects(t *testing.T) {
	tmp := t.TempDir()
	distPortal := filepath.Join(tmp, "dist", "portal", "browser")
	_ = os.MkdirAll(distPortal, 0755)
	portalStats := filepath.Join(tmp, "dist", "portal", "stats.json")
	_ = os.WriteFile(portalStats, []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(distPortal, "index.html"), []byte("<html></html>"), 0644)
	_ = os.WriteFile(filepath.Join(distPortal, "main.js"), []byte("console.log(1)"), 0644)

	targets, err := ParseAppTargets(tmp, []string{"portal"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Name != "portal" || targets[0].Stats != portalStats || targets[0].Dist != distPortal {
		t.Errorf("unexpected target: %+v", targets[0])
	}
}

func TestParseAppTargets_DuplicateNames(t *testing.T) {
	tmp := t.TempDir()
	stats1 := filepath.Join(tmp, "stats.json")
	_ = os.WriteFile(stats1, []byte("{}"), 0644)

	_, err := ParseAppTargets(tmp, nil, []string{"portal=" + stats1, "PORTAL=" + stats1})
	if err == nil {
		t.Fatal("expected error on duplicate project name")
	}
}

func TestParseAppTargets_CollidingArtifacts(t *testing.T) {
	tmp := t.TempDir()
	stats1 := filepath.Join(tmp, "stats.json")
	_ = os.WriteFile(stats1, []byte("{}"), 0644)

	_, err := ParseAppTargets(tmp, nil, []string{"portal=" + stats1, "admin=" + stats1})
	if err == nil {
		t.Fatal("expected error on colliding artifact paths")
	}
}

func TestParseAppTargets_InvalidSyntax(t *testing.T) {
	tmp := t.TempDir()
	for _, spec := range []string{"portal", "=stats.json", "portal="} {
		_, err := ParseAppTargets(tmp, nil, []string{spec})
		if err == nil {
			t.Errorf("expected error for invalid spec %q", spec)
		}
	}
}

func TestParseAppTargets_MissingStatsFile(t *testing.T) {
	tmp := t.TempDir()
	_, err := ParseAppTargets(tmp, nil, []string{"portal=" + filepath.Join(tmp, "nonexistent.json")})
	if err == nil {
		t.Fatal("expected error for missing stats file")
	}
}
