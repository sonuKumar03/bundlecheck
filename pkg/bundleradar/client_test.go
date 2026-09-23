package bundleradar_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

func TestClient_ScanAndGate(t *testing.T) {
	client := bundleradar.New()

	statsPath := filepath.Join("..", "..", "testdata", "minimal", "stats.json")
	distDir := filepath.Join("..", "..", "testdata", "minimal", "browser")

	bundle, err := client.Scan(context.Background(), bundleradar.ScanOptions{
		StatsPath: statsPath,
		DistPath:  distDir,
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if bundle == nil || len(bundle.Entrypoints) == 0 {
		t.Fatalf("expected valid scanned bundle, got %+v", bundle)
	}

	maxInitial := int64(10 * 1024 * 1024) // 10MB (passes)
	gateRes := client.Gate(bundle, nil, bundleradar.Policy{
		MaxInitial: &maxInitial,
	})

	if !gateRes.Passed {
		t.Fatalf("expected gate to pass, got violations: %+v", gateRes.Violations)
	}
}

func BenchmarkClient_Scan(b *testing.B) {
	client := bundleradar.New()
	statsPath := filepath.Join("..", "..", "testdata", "minimal", "stats.json")
	distDir := filepath.Join("..", "..", "testdata", "minimal", "browser")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.Scan(ctx, bundleradar.ScanOptions{
			StatsPath: statsPath,
			DistPath:  distDir,
		})
	}
}

func BenchmarkClient_Gate(b *testing.B) {
	client := bundleradar.New()
	statsPath := filepath.Join("..", "..", "testdata", "minimal", "stats.json")
	distDir := filepath.Join("..", "..", "testdata", "minimal", "browser")
	bundle, err := client.Scan(context.Background(), bundleradar.ScanOptions{
		StatsPath: statsPath,
		DistPath:  distDir,
	})
	if err != nil {
		b.Fatalf("setup scan failed: %v", err)
	}

	maxInitial := int64(10 * 1024 * 1024)
	pol := bundleradar.Policy{
		MaxInitial:          &maxInitial,
		DetectDuplicatePkgs: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.Gate(bundle, nil, pol)
	}
}

func BenchmarkParseBytes(b *testing.B) {
	samples := []string{"100MB", "250KB", "1.5GB", "1024B", "-50KB"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, s := range samples {
			_, _ = bundleradar.ParseBytes(s)
		}
	}
}

