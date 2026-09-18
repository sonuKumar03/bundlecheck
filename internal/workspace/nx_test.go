package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadMetadataStaticStandalone(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "nx.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "project.json"), []byte(`{
		"name": "standalone-app",
		"projectType": "application",
		"targets": {
			"build": {
				"executor": "@angular-devkit/build-angular:application"
			}
		}
	}`), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := ReadMetadataStatic(tmp)
	if err != nil {
		t.Fatalf("ReadMetadataStatic failed for standalone workspace: %v", err)
	}
	app, ok := m.Graph.Nodes["standalone-app"]
	if !ok {
		t.Fatalf("expected standalone-app in metadata nodes")
	}
	if !IsApplication(app) {
		t.Errorf("expected standalone-app to be application")
	}
	if app.Data.Root != "." {
		t.Errorf("expected standalone-app root to be '.', got %q", app.Data.Root)
	}
}

func BenchmarkReadMetadataStatic(b *testing.B) {
	wd, err := os.Getwd()
	if err != nil {
		b.Fatal(err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	testWorkspace := filepath.Join(repoRoot, "testdata", "nx-workspace")

	if _, err := os.Stat(filepath.Join(testWorkspace, "nx.json")); os.IsNotExist(err) {
		b.Skip("testdata/nx-workspace not found")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m, err := ReadMetadataStatic(testWorkspace)
		if err != nil || len(m.Graph.Nodes) == 0 {
			b.Fatalf("failed static read: %v", err)
		}
	}
}
