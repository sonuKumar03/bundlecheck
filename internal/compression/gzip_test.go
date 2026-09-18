package compression_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bundlecheck/internal/compression"
	"bundlecheck/internal/snapshot"
)

func TestMeasureBytes(t *testing.T) {
	data := []byte(strings.Repeat("console.log('hello bundlecheck world');\n", 100))
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
