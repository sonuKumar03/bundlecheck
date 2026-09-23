package compression_test

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/artifact"
	"github.com/sonuKumar03/bundleradar/internal/compression"
	"github.com/sonuKumar03/bundleradar/internal/snapshot"
)

func TestMeasureBytes(t *testing.T) {
	data := []byte(strings.Repeat("console.log('hello bundleradar world');\n", 100))
	gzSize := compression.MeasureBytes(data)

	if gzSize <= 0 || gzSize >= int64(len(data)) {
		t.Errorf("expected compression to reduce size from %d, got %d", len(data), gzSize)
	}
}

func TestMeasureFile(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "main.js")
	data := []byte(strings.Repeat("function foo() { return 42; }\n", 500))
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	gzSize, err := compression.MeasureFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gzSize <= 0 || gzSize >= int64(len(data)) {
		t.Errorf("expected gzip file size < %d, got %d", len(data), gzSize)
	}
}

func TestAttachCompression(t *testing.T) {
	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Totals: snapshot.Totals{
			InitialJS: 10000,
			LazyJS:    20000,
			TotalJS:   30000,
		},
		Outputs: []snapshot.BundleOutput{
			{
				Path:    "browser/main.js",
				Bytes:   10000,
				Initial: true,
			},
			{
				Path:    "browser/chunk-1.js",
				Bytes:   20000,
				Initial: false,
			},
		},
		Packages: []snapshot.Package{
			{
				Name:         "lodash",
				InitialBytes: 5000,
				TotalBytes:   5000,
			},
		},
	}

	compression.AttachCompression(snap, t.TempDir())

	if snap.Totals.InitialGzipJS <= 0 || snap.Totals.InitialGzipJS >= snap.Totals.InitialJS {
		t.Errorf("expected InitialGzipJS between 0 and 10000, got %d", snap.Totals.InitialGzipJS)
	}
	if snap.Totals.TotalGzipJS <= 0 || snap.Totals.TotalGzipJS >= snap.Totals.TotalJS {
		t.Errorf("expected TotalGzipJS between 0 and 30000, got %d", snap.Totals.TotalGzipJS)
	}
	if len(snap.Packages) > 0 && snap.Packages[0].InitialGzipBytes <= 0 {
		t.Errorf("expected package InitialGzipBytes > 0, got %d", snap.Packages[0].InitialGzipBytes)
	}
}

func TestCompressionUsesResolvedNestedBrowserFile(t *testing.T) {
	dist := filepath.Join(t.TempDir(), "browser")
	if err := os.MkdirAll(filepath.Join(dist, "chunks"), 0755); err != nil {
		t.Fatal(err)
	}
	data := []byte(strings.Repeat("console.log('nested chunk');", 100))
	for name, content := range map[string][]byte{
		"index.html":     []byte(`<script type="module" src="chunks/main.js"></script>`),
		"chunks/main.js": data,
		"main.js":        []byte(strings.Repeat("x", len(data))),
	} {
		if err := os.WriteFile(filepath.Join(dist, name), content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	outputs, _, err := artifact.BrowserOutputs([]snapshot.BundleOutput{{Path: "dist/app/browser/chunks/main.js", Bytes: int64(len(data))}}, dist)
	if err != nil {
		t.Fatal(err)
	}
	var expected bytes.Buffer
	w := gzip.NewWriter(&expected)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	s := &snapshot.BundleSnapshot{Outputs: outputs}
	compression.AttachCompression(s, dist)
	if got := s.Outputs[0].GzipBytes; got != int64(expected.Len()) {
		t.Fatalf("gzip measured a different file: got %d, want %d", got, expected.Len())
	}
}

func BenchmarkAttachCompressionScaling(b *testing.B) {
	const numPackages = 100
	const contributionsPerPackage = 10

	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Totals:        snapshot.Totals{InitialJS: 2000000, TotalJS: 2000000},
		Outputs: []snapshot.BundleOutput{
			{
				Path:    "browser/main.js",
				Bytes:   2000000,
				Initial: true,
			},
		},
	}

	for i := 0; i < numPackages; i++ {
		pkgName := strings.Repeat("pkg", 1) + string(rune('a'+(i%26))) + string(rune('0'+(i/26)))
		snap.Packages = append(snap.Packages, snapshot.Package{
			Name:         pkgName,
			InitialBytes: 20000,
			TotalBytes:   20000,
		})
		for j := 0; j < contributionsPerPackage; j++ {
			snap.Outputs[0].Inputs = append(snap.Outputs[0].Inputs, snapshot.Contribution{
				Input: "node_modules/" + pkgName + "/index" + string(rune('0'+j)) + ".js",
				Bytes: 2000,
			})
		}
	}

	distDir := b.TempDir()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		compression.AttachCompression(snap, distDir)
	}
}

func TestPackageCompressionPreservesContributionRoundingAndFallback(t *testing.T) {
	s := &snapshot.BundleSnapshot{
		Outputs: []snapshot.BundleOutput{
			{Path: "main.js", Bytes: 1000, Initial: true, Inputs: []snapshot.Contribution{
				{Input: "node_modules/@scope/tool/a.js", Bytes: 31},
				{Input: "node_modules/parent/node_modules/@scope/tool/b.js", Bytes: 31},
				{Input: "node_modules/other/index.js", Bytes: 100},
			}},
			{Path: "lazy.js", Bytes: 1000, Inputs: []snapshot.Contribution{
				{Input: "node_modules/@scope/tool/c.js", Bytes: 21},
			}},
		},
		Packages: []snapshot.Package{
			{Name: "@scope/tool", InitialBytes: 62, LazyBytes: 21, TotalBytes: 83},
			{Name: "OTHER", InitialBytes: 100, TotalBytes: 100},
			{Name: "missing", InitialBytes: 10, TotalBytes: 10},
		},
	}
	compression.AttachCompression(s, t.TempDir())
	for i, want := range [][3]int64{{18, 6, 24}, {32, 0, 32}, {3, 0, 3}} {
		p := s.Packages[i]
		if got := [3]int64{p.InitialGzipBytes, p.LazyGzipBytes, p.TotalGzipBytes}; got != want {
			t.Fatalf("%s gzip totals: got %v, want %v", p.Name, got, want)
		}
	}
}
