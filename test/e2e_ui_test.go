package test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sonuKumar03/bundleradar/internal/adapters/server"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

func TestE2E_StudioWebServer(t *testing.T) {
	fixtureStats := "../testdata/nx-workspace/apps/portal/stats.json"
	if _, err := os.Stat(fixtureStats); os.IsNotExist(err) {
		fixtureStats = "../testdata/nx-workspace/dist/apps/portal/stats.json"
	}

	srv, err := server.New(server.Config{
		Host:      "127.0.0.1",
		Port:      0, // random available port
		StatsPath: fixtureStats,
		Client:    bundleradar.New(),
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	baseURL := srv.URL()

	// 1. Embedded HTML shell responds with 200 OK and contains "BundleRadar"
	respHTML, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer respHTML.Body.Close()

	if respHTML.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK on /, got %d", respHTML.StatusCode)
	}
	html, err := io.ReadAll(respHTML.Body)
	if err != nil {
		t.Fatalf("read html body failed: %v", err)
	}
	if !strings.Contains(string(html), "BundleRadar") {
		t.Errorf("expected 'BundleRadar' in HTML shell")
	}

	// 2. /api/bundle returns real parsed stats from testdata/nx-workspace/apps/portal/stats.json
	respAPI, err := http.Get(baseURL + "/api/bundle")
	if err != nil {
		t.Fatalf("GET /api/bundle failed: %v", err)
	}
	defer respAPI.Body.Close()

	if respAPI.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on /api/bundle, got %d", respAPI.StatusCode)
	}

	var data server.BundleDTO
	if err := json.NewDecoder(respAPI.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode bundle json: %v", err)
	}

	if len(data.Entrypoints) == 0 {
		t.Errorf("expected non-empty entrypoints map")
	}
	if len(data.Chunks) == 0 {
		t.Errorf("expected non-empty chunks slice")
	}
	if len(data.TopPackages) == 0 {
		t.Errorf("expected non-empty topPackages slice")
	}
	if data.TotalBytes <= 0 {
		t.Errorf("expected positive totalBytes, got %d", data.TotalBytes)
	}

	// 3. /treemap.js and /app.js return 200 OK with javascript content
	for _, jsPath := range []string{"/treemap.js", "/app.js"} {
		respJS, err := http.Get(baseURL + jsPath)
		if err != nil {
			t.Fatalf("GET %s failed: %v", jsPath, err)
		}
		if respJS.StatusCode != http.StatusOK {
			_ = respJS.Body.Close()
			t.Errorf("expected 200 OK for %s, got %d", jsPath, respJS.StatusCode)
			continue
		}
		jsBody, err := io.ReadAll(respJS.Body)
		_ = respJS.Body.Close()
		if err != nil {
			t.Fatalf("failed reading %s: %v", jsPath, err)
		}
		if len(jsBody) == 0 {
			t.Errorf("expected non-empty body for %s", jsPath)
		}
	}

	// 4. Graceful server shutdown cleans up listener
	if err := srv.Close(); err != nil {
		t.Fatalf("failed to close server: %v", err)
	}

	// Verify listener is cleaned up and subsequent connection fails
	client := &http.Client{Timeout: 500 * time.Millisecond}
	respClosed, err := client.Get(baseURL + "/api/health")
	if err == nil {
		_ = respClosed.Body.Close()
		t.Errorf("expected error connecting to closed server, got status %d", respClosed.StatusCode)
	}
}
