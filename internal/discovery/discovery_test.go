package discovery_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestDiscoveryNxWorkspace(t *testing.T) {
	nxRoot := filepath.Join("..", "..", "testdata", "nx-workspace")
	if _, err := os.Stat(nxRoot); os.IsNotExist(err) {
		t.Skip("testdata/nx-workspace fixture not present")
	}
	if _, err := os.Stat(filepath.Join(nxRoot, "dist")); os.IsNotExist(err) {
		t.Skip("testdata/nx-workspace/dist build artifacts not present")
	}

	// Multi-project without filter should fail due to ambiguity
	_, _, err := discovery.Locate(nxRoot, "")
	if err == nil {
		t.Fatal("expected error due to multiple applications in nx-workspace")
	}

	// Filter by portal
	portalStats, portalDist, err := discovery.Locate(nxRoot, "portal")
	if err != nil {
		t.Fatalf("failed to locate portal: %v", err)
	}
	if !strings.Contains(portalStats, filepath.Join("dist", "apps", "portal", "stats.json")) {
		t.Errorf("unexpected portal stats: %s", portalStats)
	}
	if !strings.Contains(portalDist, filepath.Join("dist", "apps", "portal", "browser")) {
		t.Errorf("unexpected portal dist: %s", portalDist)
	}

	// Filter by admin-dashboard
	adminStats, adminDist, err := discovery.Locate(nxRoot, "admin-dashboard")
	if err != nil {
		t.Fatalf("failed to locate admin-dashboard: %v", err)
	}
	if !strings.Contains(adminStats, filepath.Join("dist", "apps", "admin-dashboard", "stats.json")) {
		t.Errorf("unexpected admin stats: %s", adminStats)
	}
	if !strings.Contains(adminDist, filepath.Join("dist", "apps", "admin-dashboard", "browser")) {
		t.Errorf("unexpected admin dist: %s", adminDist)
	}
}

func TestDiscoveryIndexCsrHtmlAndNxApps(t *testing.T) {
	tmp := t.TempDir()
	appDist := filepath.Join(tmp, "dist", "apps", "shop", "browser")
	serverDist := filepath.Join(tmp, "dist", "apps", "shop", "server")
	_ = os.MkdirAll(appDist, 0755)
	_ = os.MkdirAll(serverDist, 0755)
	_ = os.WriteFile(filepath.Join(tmp, "dist", "apps", "shop", "stats.json"), []byte(`{}`), 0644)
	_ = os.WriteFile(filepath.Join(appDist, "index.csr.html"), []byte(`<html></html>`), 0644)
	_ = os.WriteFile(filepath.Join(appDist, "main.js"), []byte(`console.log(1)`), 0644)
	_ = os.WriteFile(filepath.Join(serverDist, "index.server.html"), []byte(`<html></html>`), 0644)
	_ = os.WriteFile(filepath.Join(serverDist, "server.mjs"), []byte(`console.log(2)`), 0644)

	stats, foundDist, err := discovery.Locate(tmp, "")
	if err != nil {
		t.Fatalf("expected single browser candidate to be detected: %v", err)
	}
	if !strings.Contains(stats, filepath.Join("dist", "apps", "shop", "stats.json")) {
		t.Errorf("unexpected stats: %s", stats)
	}
	if foundDist != appDist {
		t.Errorf("expected dist %q, got %q", appDist, foundDist)
	}
}

func BenchmarkFindCandidatesScaling(b *testing.B) {
	tmp := b.TempDir()
	const numApps = 200

	for i := 0; i < numApps; i++ {
		appName := fmt.Sprintf("app-%d", i)
		appDir := filepath.Join(tmp, "dist", appName)
		browserDir := filepath.Join(appDir, "browser")
		_ = os.MkdirAll(browserDir, 0755)
		_ = os.WriteFile(filepath.Join(appDir, "stats.json"), []byte(`{}`), 0644)
		_ = os.WriteFile(filepath.Join(browserDir, "index.html"), []byte(`<html></html>`), 0644)
		_ = os.WriteFile(filepath.Join(browserDir, "main.js"), []byte(`console.log(1)`), 0644)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		candidates, err := discovery.FindCandidates(tmp)
		if err != nil || len(candidates) != numApps {
			b.Fatalf("unexpected result: count=%d, err=%v", len(candidates), err)
		}
	}
}

