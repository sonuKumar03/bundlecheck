package analysis

import (
	"reflect"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/snapshot"
)

func TestInspectChunkAggregatesAndSortsContributors(t *testing.T) {
	s := &snapshot.BundleSnapshot{SchemaVersion: "1", Outputs: []snapshot.BundleOutput{{
		Path: "browser/chunk-A.js", Bytes: 100, EntryPoint: "src/lazy.ts",
		Inputs: []snapshot.Contribution{
			{Input: "src/lazy.ts", Bytes: 10},
			{Input: "node_modules/pdfjs-dist/a.js", Bytes: 20},
			{Input: "node_modules/pdfjs-dist/b.js", Bytes: 30},
		},
	}}}

	got, err := InspectChunk(s, "chunk-A.js")
	if err != nil {
		t.Fatal(err)
	}
	if got.Chunk != "browser/chunk-A.js" || got.Bytes != 100 || got.Initial || got.EntryPoint != "src/lazy.ts" {
		t.Fatalf("chunk: %+v", got)
	}
	if want := []Contributor{{Name: "pdfjs-dist", Bytes: 50}}; !reflect.DeepEqual(got.Packages, want) {
		t.Fatalf("packages: %+v, want %+v", got.Packages, want)
	}
	wantModules := []Contributor{
		{Name: "node_modules/pdfjs-dist/b.js", Bytes: 30},
		{Name: "node_modules/pdfjs-dist/a.js", Bytes: 20},
		{Name: "src/lazy.ts", Bytes: 10},
	}
	if !reflect.DeepEqual(got.Modules, wantModules) {
		t.Fatalf("modules: %+v, want %+v", got.Modules, wantModules)
	}
}

func TestInspectChunkRejectsAmbiguousFilename(t *testing.T) {
	s := &snapshot.BundleSnapshot{Outputs: []snapshot.BundleOutput{
		{Path: "browser/chunk-A.js"},
		{Path: "server/chunk-A.js"},
	}}

	if _, err := InspectChunk(s, "chunk-A.js"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous filename error, got %v", err)
	}
}
