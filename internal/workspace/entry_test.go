package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfiguredEntry_RejectsAmbiguousConfigurations(t *testing.T) {
	root := t.TempDir()
	config := `{"projects":{"app":{"targets":{"build":{"options":{"browser":"src/main.ts"},"configurations":{"production":{"browser":"src/main.prod.ts"},"development":{"browser":"src/main.ts"}}}}}}}`
	if err := os.WriteFile(filepath.Join(root, "angular.json"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	if entry, ok := ResolveConfiguredEntry(filepath.Join(root, "dist", "app", "stats.json"), "app"); ok {
		t.Fatalf("ambiguous configuration selected %q", entry)
	}
}
