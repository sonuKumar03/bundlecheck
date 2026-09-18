package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadMetadataStatic(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	testWorkspace := filepath.Join(repoRoot, "testdata", "nx-workspace")

	if _, err := os.Stat(filepath.Join(testWorkspace, "nx.json")); os.IsNotExist(err) {
		t.Skip("testdata/nx-workspace not found")
	}

	m, err := ReadMetadataStatic(testWorkspace)
	if err != nil {
		t.Fatalf("ReadMetadataStatic failed: %v", err)
	}

	if len(m.Graph.Nodes) == 0 {
		t.Fatalf("expected discovered nodes, got 0")
	}

	admin, ok := m.Graph.Nodes["admin-dashboard"]
	if !ok {
		t.Fatalf("expected admin-dashboard project in static metadata")
	}

	if !IsApplication(admin) {
		t.Errorf("expected admin-dashboard to be an application")
	}

	if admin.Data.Root != filepath.FromSlash("apps/admin-dashboard") && admin.Data.Root != "apps/admin-dashboard" {
		t.Errorf("expected root apps/admin-dashboard, got %q", admin.Data.Root)
	}

	portal, ok := m.Graph.Nodes["portal"]
	if !ok {
		t.Fatalf("expected portal project in static metadata")
	}

	if !IsApplication(portal) {
		t.Errorf("expected portal to be an application")
	}

	charting, ok := m.Graph.Nodes["charting"]
	if !ok {
		t.Fatalf("expected charting project in static metadata")
	}
	if IsApplication(charting) {
		t.Errorf("expected charting to be a library, not application")
	}
}

func TestReadMetadataStaticFallbackCases(t *testing.T) {
	t.Run("missing nx.json", func(t *testing.T) {
		tmp := t.TempDir()
		_, err := ReadMetadataStatic(tmp)
		if err == nil {
			t.Fatal("expected error for missing nx.json")
		}
	})

	t.Run("no projects", func(t *testing.T) {
		tmp := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmp, "nx.json"), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := ReadMetadataStatic(tmp)
		if !errors.Is(err, ErrStaticFallbackRequired) {
			t.Fatalf("expected ErrStaticFallbackRequired, got %v", err)
		}
	})

	t.Run("only library projects", func(t *testing.T) {
		tmp := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmp, "nx.json"), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
		libDir := filepath.Join(tmp, "libs", "my-lib")
		if err := os.MkdirAll(libDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(libDir, "project.json"), []byte(`{"name":"my-lib","projectType":"library"}`), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := ReadMetadataStatic(tmp)
		if !errors.Is(err, ErrStaticFallbackRequired) {
			t.Fatalf("expected ErrStaticFallbackRequired, got %v", err)
		}
	})

	t.Run("app with unsupported executor", func(t *testing.T) {
		tmp := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmp, "nx.json"), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
		appDir := filepath.Join(tmp, "apps", "custom-app")
		if err := os.MkdirAll(appDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(appDir, "project.json"), []byte(`{
			"name": "custom-app",
			"projectType": "application",
			"targets": {
				"build": {
					"executor": "@nx/webpack:webpack"
				}
			}
		}`), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := ReadMetadataStatic(tmp)
		if !errors.Is(err, ErrStaticFallbackRequired) {
			t.Fatalf("expected ErrStaticFallbackRequired, got %v", err)
		}
	})

	t.Run("app in ignored directories", func(t *testing.T) {
		tmp := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmp, "nx.json"), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
		for _, ignored := range []string{"node_modules", "dist", ".git", ".nx", "coverage", "tmp"} {
			appDir := filepath.Join(tmp, ignored, "app")
			if err := os.MkdirAll(appDir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(appDir, "project.json"), []byte(`{
				"name": "ignored-app",
				"projectType": "application",
				"targets": {
					"build": {
						"executor": "@angular-devkit/build-angular:application"
					}
				}
			}`), 0644); err != nil {
				t.Fatal(err)
			}
		}
		_, err := ReadMetadataStatic(tmp)
		if !errors.Is(err, ErrStaticFallbackRequired) {
			t.Fatalf("expected ErrStaticFallbackRequired, got %v", err)
		}
	})
}
