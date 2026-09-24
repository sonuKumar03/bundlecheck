package parsers_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/parsers"
	"github.com/sonuKumar03/bundleradar/internal/core"
)

type PortalGroundTruth struct {
	InitialFiles      map[string]int64
	InitialGzipBytes  map[string]int64
	LazyFiles         map[string]int64
	LazyGzipBytes     map[string]int64
	TotalInitialBytes int64
	TotalInitialGzip  int64
	TotalLazyBytes    int64
	TotalLazyGzip     int64
}

// computeRealGzip compresses bytes with standard gzip level and returns byte length.
func computeRealGzip(data []byte) (int64, error) {
	var buf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
	if err != nil {
		return 0, err
	}
	if _, err := gw.Write(data); err != nil {
		return 0, err
	}
	if err := gw.Close(); err != nil {
		return 0, err
	}
	return int64(buf.Len()), nil
}

// extractPortalGroundTruth reads index.html and disk files to establish zero-assumption ground truth.
func extractPortalGroundTruth(t *testing.T, distDir string) *PortalGroundTruth {
	t.Helper()

	indexPath := filepath.Join(distDir, "index.html")
	indexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read portal index.html: %v", err)
	}
	indexContent := string(indexBytes)

	gt := &PortalGroundTruth{
		InitialFiles:     make(map[string]int64),
		InitialGzipBytes: make(map[string]int64),
		LazyFiles:        make(map[string]int64),
		LazyGzipBytes:    make(map[string]int64),
	}

	// 1. Extract <script src="...">
	scriptRe := regexp.MustCompile(`<script[^>]+src=["']([^"']+)["']`)
	for _, match := range scriptRe.FindAllStringSubmatch(indexContent, -1) {
		name := filepath.Base(match[1])
		gt.InitialFiles[name] = 0
	}

	// 2. Extract <link rel="stylesheet" href="...">
	linkRe := regexp.MustCompile(`<link[^>]+rel=["']stylesheet["'][^>]+href=["']([^"']+)["']`)
	for _, match := range linkRe.FindAllStringSubmatch(indexContent, -1) {
		name := filepath.Base(match[1])
		gt.InitialFiles[name] = 0
	}

	// Also catch reversed attribute order: href then rel
	linkReRev := regexp.MustCompile(`<link[^>]+href=["']([^"']+)["'][^>]+rel=["']stylesheet["']`)
	for _, match := range linkReRev.FindAllStringSubmatch(indexContent, -1) {
		name := filepath.Base(match[1])
		gt.InitialFiles[name] = 0
	}

	// 3. Extract <link rel="modulepreload" href="...">
	preloadRe := regexp.MustCompile(`<link[^>]+href=["']([^"']+)["'][^>]+rel=["']modulepreload["']`)
	for _, match := range preloadRe.FindAllStringSubmatch(indexContent, -1) {
		name := filepath.Base(match[1])
		gt.InitialFiles[name] = 0
	}
	preloadReRev := regexp.MustCompile(`<link[^>]+rel=["']modulepreload["'][^>]+href=["']([^"']+)["']`)
	for _, match := range preloadReRev.FindAllStringSubmatch(indexContent, -1) {
		name := filepath.Base(match[1])
		gt.InitialFiles[name] = 0
	}

	// 4. Measure physical file sizes & gzip on disk for all files in distDir
	entries, err := os.ReadDir(distDir)
	if err != nil {
		t.Fatalf("failed to read distDir %q: %v", distDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))

		// Only JS and CSS are code/style bundles
		if ext != ".js" && ext != ".mjs" && ext != ".css" {
			continue
		}

		fullPath := filepath.Join(distDir, name)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("failed to read file %q: %v", fullPath, err)
		}

		rawSize := int64(len(content))
		gzSize, err := computeRealGzip(content)
		if err != nil {
			t.Fatalf("failed to compute gzip for %q: %v", fullPath, err)
		}

		if _, isInitial := gt.InitialFiles[name]; isInitial {
			gt.InitialFiles[name] = rawSize
			gt.InitialGzipBytes[name] = gzSize
			gt.TotalInitialBytes += rawSize
			gt.TotalInitialGzip += gzSize
		} else {
			// Lazy chunk
			gt.LazyFiles[name] = rawSize
			gt.LazyGzipBytes[name] = gzSize
			gt.TotalLazyBytes += rawSize
			gt.TotalLazyGzip += gzSize
		}
	}

	return gt
}

func TestPortalGroundTruthBaseline(t *testing.T) {
	distDir := filepath.Join("..", "..", "..", "testdata", "nx-workspace", "dist", "apps", "portal", "browser")
	if _, err := os.Stat(filepath.Join(distDir, "index.html")); os.IsNotExist(err) {
		t.Skipf("skipping: portal dist not present at %s", distDir)
	}

	gt := extractPortalGroundTruth(t, distDir)

	t.Logf("=== DISK GROUND TRUTH BASELINE (portal) ===")
	t.Logf("Initial files count: %d", len(gt.InitialFiles))
	for f, size := range gt.InitialFiles {
		t.Logf("  [INITIAL] %-25s raw: %8d B, gzip: %6d B", f, size, gt.InitialGzipBytes[f])
	}
	t.Logf("Total Initial Raw: %d bytes (%.2f MB)", gt.TotalInitialBytes, float64(gt.TotalInitialBytes)/(1024*1024))
	t.Logf("Total Initial Gzip: %d bytes (%.2f KB)", gt.TotalInitialGzip, float64(gt.TotalInitialGzip)/1024)

	t.Logf("Lazy files count: %d", len(gt.LazyFiles))
	for f, size := range gt.LazyFiles {
		t.Logf("  [LAZY]    %-25s raw: %8d B, gzip: %6d B", f, size, gt.LazyGzipBytes[f])
	}
	t.Logf("Total Lazy Raw: %d bytes (%.2f KB)", gt.TotalLazyBytes, float64(gt.TotalLazyBytes)/1024)
	t.Logf("Total Lazy Gzip: %d bytes (%.2f KB)", gt.TotalLazyGzip, float64(gt.TotalLazyGzip)/1024)

	// Invariants derived from physical index.html:
	// Initial files MUST include main, polyfills, styles, and the 6 modulepreload vendor/runtime chunks = 9 files
	if len(gt.InitialFiles) != 9 {
		t.Errorf("expected exactly 9 initial files referenced by index.html, got %d", len(gt.InitialFiles))
	}

	// Lazy chunks MUST be 5 chunks
	if len(gt.LazyFiles) != 5 {
		t.Errorf("expected exactly 5 lazy chunks not in index.html, got %d", len(gt.LazyFiles))
	}
}

func TestAngularParser_InitialVsAsyncBoundary(t *testing.T) {
	distDir := filepath.Join("..", "..", "..", "testdata", "nx-workspace", "dist", "apps", "portal", "browser")
	statsPath := filepath.Join("..", "..", "..", "testdata", "nx-workspace", "dist", "apps", "portal", "stats.json")

	if _, err := os.Stat(statsPath); os.IsNotExist(err) {
		t.Skipf("skipping: portal stats not present at %s", statsPath)
	}

	gt := extractPortalGroundTruth(t, distDir)

	parser := &parsers.AngularParser{}
	target := core.Target{
		Name:      "portal",
		StatsPath: statsPath,
		DistPath:  distDir,
	}

	bundle, err := parser.Parse(context.Background(), target)
	if err != nil {
		t.Fatalf("parser.Parse failed: %v", err)
	}

	// 1. Verify every initial chunk in bundle is physically an initial file in index.html
	var bundleInitialBytes int64
	var bundleAsyncBytes int64

	for _, chunk := range bundle.Chunks {
		if chunk.Type == core.LoadTypeInitial {
			bundleInitialBytes += chunk.SizeBytes
			if _, isInitial := gt.InitialFiles[chunk.Name]; !isInitial {
				t.Errorf("chunk %q marked as INITIAL by parser, but is NOT in index.html ground truth", chunk.Name)
			}
		} else if chunk.Type == core.LoadTypeAsync {
			bundleAsyncBytes += chunk.SizeBytes
			if _, isLazy := gt.LazyFiles[chunk.Name]; !isLazy {
				t.Errorf("chunk %q marked as ASYNC by parser, but is NOT in lazy chunks ground truth", chunk.Name)
			}
		}
	}

	// Also check initial CSS assets
	var initialCSSBytes int64
	for _, asset := range bundle.Assets {
		if strings.HasSuffix(asset.Path, ".css") {
			baseName := filepath.Base(asset.Path)
			if _, isInitial := gt.InitialFiles[baseName]; isInitial {
				initialCSSBytes += asset.SizeBytes
			}
		}
	}

	totalMeasuredInitial := bundleInitialBytes + initialCSSBytes

	t.Logf("Ground Truth Initial Bytes: %d, Parser Initial Bytes: %d (JS: %d, CSS: %d)",
		gt.TotalInitialBytes, totalMeasuredInitial, bundleInitialBytes, initialCSSBytes)
	t.Logf("Ground Truth Lazy Bytes:    %d, Parser Lazy Bytes:    %d",
		gt.TotalLazyBytes, bundleAsyncBytes)

	if totalMeasuredInitial != gt.TotalInitialBytes {
		t.Errorf("initial bytes mismatch: got %d, want ground truth %d", totalMeasuredInitial, gt.TotalInitialBytes)
	}

	if bundleAsyncBytes != gt.TotalLazyBytes {
		t.Errorf("async bytes mismatch: got %d, want ground truth %d", bundleAsyncBytes, gt.TotalLazyBytes)
	}
}

func TestAngularParser_PackageAttribution(t *testing.T) {
	distDir := filepath.Join("..", "..", "..", "testdata", "nx-workspace", "dist", "apps", "portal", "browser")
	statsPath := filepath.Join("..", "..", "..", "testdata", "nx-workspace", "dist", "apps", "portal", "stats.json")

	if _, err := os.Stat(statsPath); os.IsNotExist(err) {
		t.Skipf("skipping: portal stats not present at %s", statsPath)
	}

	parser := &parsers.AngularParser{}
	target := core.Target{
		Name:      "portal",
		StatsPath: statsPath,
		DistPath:  distDir,
	}

	bundle, err := parser.Parse(context.Background(), target)
	if err != nil {
		t.Fatalf("parser.Parse failed: %v", err)
	}

	// 1. Verify module deduplication: every module ID must be unique
	seenIDs := make(map[string]bool)
	for _, m := range bundle.Modules {
		if seenIDs[m.ID] {
			t.Fatalf("duplicate module found in bundle.Modules: %q", m.ID)
		}
		seenIDs[m.ID] = true

		// Verify first-party vs third-party attribution
		if strings.Contains(m.ID, "node_modules/") {
			if m.IsAppCode {
				t.Errorf("module %q from node_modules incorrectly marked as IsAppCode", m.ID)
			}
			if m.Package == "" {
				t.Errorf("module %q from node_modules has empty Package name", m.ID)
			}
		} else {
			if !m.IsAppCode {
				t.Errorf("first-party module %q incorrectly marked as third-party", m.ID)
			}
		}
		if len(m.ChunkIDs) == 0 {
			t.Errorf("module %q has no associated ChunkIDs", m.ID)
		}
	}

	// 2. Verify TopPackages rankings
	top := bundle.TopPackages(5)
	if len(top) < 5 {
		t.Fatalf("expected at least 5 top packages, got %d", len(top))
	}

	expectedPackages := map[string]bool{
		"exceljs":           true,
		"pdfjs-dist":        true,
		"chart.js":          true,
		"@angular/material": true,
		"@angular/core":     true,
	}

	t.Logf("=== TOP 5 CONTRIBUTING NPM PACKAGES ===")
	for i, pkg := range top {
		t.Logf("  #%d %-22s %10d bytes (~%8d gzip) in %3d modules",
			i+1, pkg.Name, pkg.SizeBytes, pkg.GzipBytes, pkg.ModuleCount)
		if !expectedPackages[pkg.Name] {
			t.Errorf("unexpected package in top 5: %q", pkg.Name)
		}
	}
}

func TestAngularParser_GzipPrecision(t *testing.T) {
	distDir := filepath.Join("..", "..", "..", "testdata", "nx-workspace", "dist", "apps", "portal", "browser")
	statsPath := filepath.Join("..", "..", "..", "testdata", "nx-workspace", "dist", "apps", "portal", "stats.json")

	if _, err := os.Stat(statsPath); os.IsNotExist(err) {
		t.Skipf("skipping: portal stats not present at %s", statsPath)
	}

	gt := extractPortalGroundTruth(t, distDir)

	parser := &parsers.AngularParser{}
	target := core.Target{
		Name:      "portal",
		StatsPath: statsPath,
		DistPath:  distDir,
	}

	bundle, err := parser.Parse(context.Background(), target)
	if err != nil {
		t.Fatalf("parser.Parse failed: %v", err)
	}

	ep, ok := bundle.Entrypoints["main"]
	if !ok {
		t.Fatalf("expected entrypoint 'main' in bundle")
	}

	// Compare parser estimated initial gzip vs ground truth real disk gzip
	diffBytes := ep.InitialGzipBytes - gt.TotalInitialGzip
	if diffBytes < 0 {
		diffBytes = -diffBytes
	}
	errorRatio := float64(diffBytes) / float64(gt.TotalInitialGzip)

	t.Logf("=== GZIP WIRE ESTIMATE VS REAL DISK GZIP ===")
	t.Logf("Real Physical Gzip:  %d bytes (%.2f KB)", gt.TotalInitialGzip, float64(gt.TotalInitialGzip)/1024)
	t.Logf("Parser Heuristic:    %d bytes (%.2f KB)", ep.InitialGzipBytes, float64(ep.InitialGzipBytes)/1024)
	t.Logf("Delta:               %d bytes (%.2f%% difference)", diffBytes, errorRatio*100)

	// In minified/bundled JS+CSS, gzip heuristic (30%) should be within 10% of physical wire transfer
	if errorRatio > 0.10 {
		t.Errorf("gzip estimate variance too high: %.2f%% (max allowed 10%%)", errorRatio*100)
	}
}



