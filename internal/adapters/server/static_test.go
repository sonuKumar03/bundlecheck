package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/server"
)

func TestServer_ServeIndexHTML(t *testing.T) {
	srv, err := server.New(server.Config{
		Host: "127.0.0.1",
		Port: 0,
	})
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	t.Run("serves root index.html", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/")
		if err != nil {
			t.Fatalf("GET / failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body failed: %v", err)
		}

		if !strings.Contains(string(body), "BundleRadar Studio") {
			t.Errorf("expected 'BundleRadar Studio' in HTML, got: %s", string(body)[:min(len(body), 200)])
		}
	})

	t.Run("serves style.css", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/style.css")
		if err != nil {
			t.Fatalf("GET /style.css failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body failed: %v", err)
		}

		if !strings.Contains(string(body), "BundleRadar Live Studio Styles") {
			t.Errorf("expected CSS content, got: %s", string(body))
		}
	})

	t.Run("serves treemap.js", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/treemap.js")
		if err != nil {
			t.Fatalf("GET /treemap.js failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body failed: %v", err)
		}

		if !strings.Contains(string(body), "class TreemapEngine") {
			t.Errorf("expected 'class TreemapEngine' in treemap.js, got: %s", string(body)[:min(len(body), 200)])
		}
	})

	t.Run("serves app.js", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/app.js")
		if err != nil {
			t.Fatalf("GET /app.js failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body failed: %v", err)
		}

		if !strings.Contains(string(body), "renderBundle") {
			t.Errorf("expected 'renderBundle' in app.js, got: %s", string(body)[:min(len(body), 200)])
		}
	})

	t.Run("returns 404 for unknown api routes", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/unknown-endpoint")
		if err != nil {
			t.Fatalf("GET /api/unknown-endpoint failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", resp.StatusCode)
		}
	})
}
