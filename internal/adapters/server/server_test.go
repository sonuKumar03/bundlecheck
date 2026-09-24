package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/server"
	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

func TestServer_HealthAndRouting(t *testing.T) {
	client := bundleradar.New()
	srv, err := server.New(server.Config{
		Host:   "127.0.0.1",
		Port:   0,
		Client: client,
	})
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var data map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if data["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", data["status"])
	}
	if data["version"] != bundleradar.ToolVersion {
		t.Errorf("expected version %q, got %q", bundleradar.ToolVersion, data["version"])
	}
}

func TestServer_CORS(t *testing.T) {
	srv, err := server.New(server.Config{})
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/api/health", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS /api/health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204 No Content for OPTIONS, got %d", resp.StatusCode)
	}
	if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin '*', got %q", origin)
	}
}

func TestServer_Lifecycle(t *testing.T) {
	srv, err := server.New(server.Config{
		Host: "127.0.0.1",
		Port: 0,
	})
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start failed: %v", err)
	}
	defer func() {
		_ = srv.Close()
	}()

	url := srv.URL()
	if url == "" {
		t.Fatal("expected non-empty URL")
	}

	resp, err := http.Get(url + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health on running server failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	if err := srv.Close(); err != nil {
		t.Fatalf("srv.Close failed: %v", err)
	}

	// Test SetBundle and Bundle
	if srv.Bundle() != nil {
		t.Errorf("expected initial bundle to be nil")
	}
	b := &core.Bundle{Metadata: core.Metadata{Bundler: "test"}}
	srv.SetBundle(b)
	if srv.Bundle() != b {
		t.Errorf("expected bundle %v, got %v", b, srv.Bundle())
	}
}
