package discovery_test

import (
	"os"
	"path/filepath"
	"testing"

	"bundlecheck/internal/discovery"
)

func TestDiscoverySingleProject(t *testing.T) {
	tmp := t.TempDir()
	dist := filepath.Join(tmp, "dist", "my-app", "browser")
	if err := os.MkdirAll(dist, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "dist", "my-app", "stats.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte(`<html></html>`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "main.js"), []byte(`console.log(1)`), 0644); err != nil {
		t.Fatal(err)
	}

	stats, foundDist, err := discovery.Locate(tmp, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats != filepath.Join(tmp, "dist", "my-app", "stats.json") {
		t.Errorf("expected stats path %q, got %q", filepath.Join(tmp, "dist", "my-app", "stats.json"), stats)
	}
	if foundDist != dist {
		t.Errorf("expected dist path %q, got %q", dist, foundDist)
	}
}

func TestDiscoveryMultiProjectWithFilter(t *testing.T) {
	tmp := t.TempDir()

	// App 1
	app1Dist := filepath.Join(tmp, "dist", "app-one", "browser")
	_ = os.MkdirAll(app1Dist, 0755)
	_ = os.WriteFile(filepath.Join(tmp, "dist", "app-one", "stats.json"), []byte(`{}`), 0644)
	_ = os.WriteFile(filepath.Join(app1Dist, "index.html"), []byte(`<html></html>`), 0644)
	_ = os.WriteFile(filepath.Join(app1Dist, "main.js"), []byte(`console.log(1)`), 0644)

	// App 2
	app2Dist := filepath.Join(tmp, "dist", "app-two", "browser")
	_ = os.MkdirAll(app2Dist, 0755)
	_ = os.WriteFile(filepath.Join(tmp, "dist", "app-two", "stats.json"), []byte(`{}`), 0644)
	_ = os.WriteFile(filepath.Join(app2Dist, "index.html"), []byte(`<html></html>`), 0644)
	_ = os.WriteFile(filepath.Join(app2Dist, "main.js"), []byte(`console.log(2)`), 0644)

	// Without project filter: should error due to ambiguity
	_, _, err := discovery.Locate(tmp, "")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}

	// With project filter: app-one
	stats1, dist1, err := discovery.Locate(tmp, "app-one")
	if err != nil {
		t.Fatalf("unexpected error for app-one: %v", err)
	}
	if dist1 != app1Dist || stats1 != filepath.Join(tmp, "dist", "app-one", "stats.json") {
		t.Errorf("mismatch for app-one: stats=%s, dist=%s", stats1, dist1)
	}

	// Non-existent project
	_, _, err = discovery.Locate(tmp, "non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent project")
	}
}

func TestDiscoveryMissing(t *testing.T) {
	tmp := t.TempDir()
	_, _, err := discovery.Locate(tmp, "")
	if err == nil {
		t.Fatal("expected error when no build artifacts exist")
	}
}
