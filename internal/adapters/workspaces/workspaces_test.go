package workspaces_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/workspaces"
)

func TestExplicitResolver(t *testing.T) {
	resolver := &workspaces.ExplicitResolver{
		Specs: []string{"portal=dist/portal/stats.json:dist/portal/browser", "admin=dist/admin/stats.json"},
	}

	targets, err := resolver.Resolve(context.Background(), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	if targets[0].Name != "portal" || targets[0].StatsPath != "dist/portal/stats.json" || targets[0].DistPath != "dist/portal/browser" {
		t.Fatalf("unexpected target 0: %+v", targets[0])
	}
	if targets[1].Name != "admin" || targets[1].StatsPath != "dist/admin/stats.json" {
		t.Fatalf("unexpected target 1: %+v", targets[1])
	}
}

func TestMonorepoResolver(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock a pnpm workspace
	pnpmYaml := `packages:
  - 'apps/*'
`
	if err := os.WriteFile(filepath.Join(tmpDir, "pnpm-workspace.yaml"), []byte(pnpmYaml), 0644); err != nil {
		t.Fatalf("failed to write pnpm-workspace.yaml: %v", err)
	}

	// Create apps/web with build output
	webDir := filepath.Join(tmpDir, "apps", "web")
	_ = os.MkdirAll(filepath.Join(webDir, "dist"), 0755)
	_ = os.WriteFile(filepath.Join(webDir, "dist", "stats.json"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(webDir, "package.json"), []byte(`{"name": "@myorg/web"}`), 0644)

	reg := workspaces.DefaultRegistry()
	targets, err := reg.Resolve(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("unexpected error resolving monorepo: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target discovered, got %d", len(targets))
	}

	if targets[0].Name != "web" && targets[0].Name != "@myorg/web" {
		t.Fatalf("unexpected discovered target name: %q", targets[0].Name)
	}
}

func TestNxResolver(t *testing.T) {
	nxDir := filepath.Join("..", "..", "..", "testdata", "nx-workspace")
	resolver := &workspaces.NxResolver{}

	if !resolver.Detect(nxDir) {
		t.Fatalf("expected nx resolver to detect nx-workspace")
	}

	targets, err := resolver.Resolve(context.Background(), nxDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) < 2 {
		t.Fatalf("expected at least 2 targets from nx-workspace, got %d", len(targets))
	}
}
