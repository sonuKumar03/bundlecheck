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
