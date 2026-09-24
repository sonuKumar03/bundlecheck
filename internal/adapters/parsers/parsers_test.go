package parsers_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/parsers"
	"github.com/sonuKumar03/bundleradar/internal/core"
)

func TestParsersRegistry_AutoDetectEsbuild(t *testing.T) {
	reg := parsers.DefaultRegistry()
	statsPath := filepath.Join("..", "..", "..", "testdata", "minimal", "stats.json")
	distDir := filepath.Join("..", "..", "..", "testdata", "minimal", "browser")

	target := core.Target{
		Name:      "minimal",
		StatsPath: statsPath,
		DistPath:  distDir,
	}

	p, err := reg.Resolve(target)
	if err != nil {
		t.Fatalf("failed to resolve parser for minimal stats: %v", err)
	}

	// Angular is built on top of esbuild with browser conventions, so it detects as angular or esbuild
	if p.Name() != "angular" && p.Name() != "esbuild" {
		t.Fatalf("expected angular or esbuild parser, got %q", p.Name())
	}

	bundle, err := p.Parse(context.Background(), target)
	if err != nil {
		t.Fatalf("failed to parse bundle: %v", err)
	}

	if len(bundle.Entrypoints) == 0 {
		t.Fatalf("expected at least 1 entrypoint, got 0")
	}

	if bundle.TotalInitialBytes() == 0 {
		t.Fatalf("expected non-zero initial bytes")
	}
}

func TestParsersRegistry_ViteManifest(t *testing.T) {
	reg := parsers.DefaultRegistry()

	// Create a temporary mock Vite manifest
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	manifestContent := `{
		"src/main.ts": {
			"file": "assets/main-1234.js",
			"src": "src/main.ts",
			"isEntry": true,
			"imports": ["_vendor-5678.js"],
			"dynamicImports": ["src/routes/about.ts"]
		},
		"_vendor-5678.js": {
			"file": "assets/vendor-5678.js"
		},
		"src/routes/about.ts": {
			"file": "assets/about-9012.js",
			"src": "src/routes/about.ts",
			"isDynamicEntry": true
		}
	}`

	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write mock manifest: %v", err)
	}

	// Also write dummy asset files to get realistic byte sizes
	assetsDir := filepath.Join(tmpDir, "assets")
	_ = os.MkdirAll(assetsDir, 0755)
	_ = os.WriteFile(filepath.Join(assetsDir, "main-1234.js"), []byte("console.log('main');"), 0644)
	_ = os.WriteFile(filepath.Join(assetsDir, "vendor-5678.js"), []byte("console.log('vendor');"), 0644)
	_ = os.WriteFile(filepath.Join(assetsDir, "about-9012.js"), []byte("console.log('about');"), 0644)

	target := core.Target{
		Name:      "vite-app",
		StatsPath: manifestPath,
		DistPath:  tmpDir,
	}

	p, err := reg.Resolve(target)
	if err != nil {
		t.Fatalf("failed to resolve parser: %v", err)
	}
	if p.Name() != "vite" {
		t.Fatalf("expected vite parser, got %s", p.Name())
	}

	bundle, err := p.Parse(context.Background(), target)
	if err != nil {
		t.Fatalf("failed to parse vite bundle: %v", err)
	}

	if _, ok := bundle.Entrypoints["src/main.ts"]; !ok {
		t.Fatalf("expected entrypoint src/main.ts in bundle")
	}

	if len(bundle.Chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(bundle.Chunks))
	}
}

func TestParsersRegistry_WebpackStats(t *testing.T) {
	reg := parsers.DefaultRegistry()

	tmpDir := t.TempDir()
	statsPath := filepath.Join(tmpDir, "webpack-stats.json")
	statsContent := `{
		"assetsByChunkName": {
			"main": ["main.bundle.js"],
			"vendor": ["vendor.bundle.js"]
		},
		"chunks": [
			{
				"id": 0,
				"names": ["main"],
				"files": ["main.bundle.js"],
				"size": 45000,
				"initial": true,
				"entry": true,
				"modules": [
					{
						"name": "./src/index.js",
						"size": 15000
					},
					{
						"name": "./node_modules/lodash/lodash.js",
						"size": 30000
					}
				]
			}
		]
	}`

	if err := os.WriteFile(statsPath, []byte(statsContent), 0644); err != nil {
		t.Fatalf("failed to write mock webpack stats: %v", err)
	}

	target := core.Target{
		Name:      "webpack-app",
		StatsPath: statsPath,
		DistPath:  tmpDir,
	}

	p, err := reg.Resolve(target)
	if err != nil {
		t.Fatalf("failed to resolve parser: %v", err)
	}
	if p.Name() != "webpack" {
		t.Fatalf("expected webpack parser, got %s", p.Name())
	}

	bundle, err := p.Parse(context.Background(), target)
	if err != nil {
		t.Fatalf("failed to parse webpack bundle: %v", err)
	}

	if _, ok := bundle.Entrypoints["main"]; !ok {
		t.Fatalf("expected main entrypoint in webpack bundle")
	}

	top := bundle.TopPackages(5)
	if len(top) == 0 || top[0].Name != "lodash" {
		t.Fatalf("expected lodash in top packages: %+v", top)
	}
}

func TestParsersRegistry_RealAngularNxApps(t *testing.T) {
	reg := parsers.DefaultRegistry()
	workspaceRoot := filepath.Join("..", "..", "..", "testdata", "nx-workspace")

	testCases := []struct {
		name         string
		statsRelPath string
		distRelPath  string
		expectedName string
	}{
		{
			name:         "portal",
			statsRelPath: filepath.Join("dist", "apps", "portal", "stats.json"),
			distRelPath:  filepath.Join("dist", "apps", "portal", "browser"),
			expectedName: "angular",
		},
		{
			name:         "admin-dashboard",
			statsRelPath: filepath.Join("dist", "apps", "admin-dashboard", "stats.json"),
			distRelPath:  filepath.Join("dist", "apps", "admin-dashboard", "browser"),
			expectedName: "angular",
		},
	}

	for _, tc := range testCases {
		statsPath := filepath.Join(workspaceRoot, tc.statsRelPath)
		distPath := filepath.Join(workspaceRoot, tc.distRelPath)

		if _, err := os.Stat(statsPath); os.IsNotExist(err) {
			t.Skipf("skipping %s: build artifact not found at %s", tc.name, statsPath)
		}

		target := core.Target{
			Name:      tc.name,
			StatsPath: statsPath,
			DistPath:  distPath,
		}

		p, err := reg.Resolve(target)
		if err != nil {
			t.Fatalf("[%s] failed to resolve parser: %v", tc.name, err)
		}

		if p.Name() != tc.expectedName {
			t.Fatalf("[%s] expected parser %q, got %q", tc.name, tc.expectedName, p.Name())
		}

		bundle, err := p.Parse(context.Background(), target)
		if err != nil {
			t.Fatalf("[%s] failed to parse bundle: %v", tc.name, err)
		}

		if len(bundle.Entrypoints) == 0 {
			t.Errorf("[%s] expected at least 1 entrypoint, got 0", tc.name)
		}

		if bundle.TotalInitialBytes() == 0 {
			t.Errorf("[%s] expected non-zero initial bytes", tc.name)
		}
	}
}

