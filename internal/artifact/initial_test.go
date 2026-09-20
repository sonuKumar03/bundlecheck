package artifact

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

func TestBrowserOutputs(t *testing.T) {
	for _, tt := range []struct {
		name, html                           string
		paths, files, wantRoots, wantOutputs []string
		wantError                            string
	}{
		{"flat stats", `<script src="main.js"></script><script src="https://cdn.example/a.js"></script><script>console.log(1)</script>`, []string{"main.js", "lazy.mjs", "styles.css"}, []string{"main.js", "lazy.mjs"}, []string{"main.js"}, []string{"main.js", "lazy.mjs"}, ""},
		{"base and URL suffix", `<base href="/app/"><script type="module" src="/app/sub/main%20file.js?v=1#x"></script>`, []string{`browser\sub\main file.js`}, []string{"sub/main file.js"}, []string{"browser/sub/main file.js"}, []string{"browser/sub/main file.js"}, ""},
		{"relative base", `<base href="app/"><script src="main.js"></script>`, []string{"main.js"}, []string{"main.js"}, []string{"main.js"}, []string{"main.js"}, ""},
		{"server excluded", `<script src="main.js"></script>`, []string{"dist/app/browser/main.js", "dist/app/server/main.js", "dist/app/browser/lazy.cjs"}, []string{"main.js", "lazy.cjs"}, []string{"dist/app/browser/main.js"}, []string{"dist/app/browser/main.js", "dist/app/browser/lazy.cjs"}, ""},
		{"multiple roots", `<SCRIPT SRC='main.js'></SCRIPT><script src="polyfills.js"></script><script src="main.js"></script>`, []string{"main.js", "polyfills.js"}, []string{"main.js", "polyfills.js"}, []string{"main.js", "polyfills.js"}, []string{"main.js", "polyfills.js"}, ""},
		{"remote base", `<base href="https://cdn.example/"><script src="main.js"></script>`, []string{"main.js"}, []string{"main.js"}, nil, nil, "bootstrap"},
		{"no scripts", `<script type="application/ld+json">{}</script>`, []string{"main.js"}, []string{"main.js"}, nil, nil, "bootstrap"},
		{"classic JS MIME type", `<script type="text/ecmascript" src="main.js"></script>`, []string{"main.js"}, []string{"main.js"}, []string{"main.js"}, []string{"main.js"}, ""},
		{"parameterized type is data", `<script src="main.js"></script><script type="text/javascript; charset=utf-8" src="lazy.js"></script>`, []string{"main.js", "lazy.js"}, []string{"main.js", "lazy.js"}, []string{"main.js"}, []string{"main.js", "lazy.js"}, ""},
		{"missing root", `<script src="missing.js"></script>`, []string{"main.js"}, []string{"main.js"}, nil, nil, "missing.js"},
		{"missing stats root", `<script src="main.js"></script>`, []string{"lazy.js"}, []string{"main.js", "lazy.js"}, nil, nil, "main.js"},
		{"ambiguous", `<script src="main.js"></script>`, []string{"one/browser/main.js", "two/browser/main.js"}, []string{"main.js"}, nil, nil, "ambiguous"},
		{"ambiguous relative prefix", `<script src="main.js"></script>`, []string{"browser/main.js"}, []string{"main.js", "browser/main.js"}, nil, nil, "ambiguous"},
		{"relative directory named browser", `<script src="browser/main.js"></script>`, []string{"browser/main.js"}, []string{"browser/main.js"}, []string{"browser/main.js"}, []string{"browser/main.js"}, ""},
		{"repeated prefix with unique match", `<script src="sub/browser/main.js"></script>`, []string{"old/browser/sub/browser/main.js"}, []string{"sub/browser/main.js"}, []string{"old/browser/sub/browser/main.js"}, []string{"old/browser/sub/browser/main.js"}, ""},
		{"repeated prefix with ambiguous matches", `<script src="main.js"></script>`, []string{"old/browser/sub/browser/main.js"}, []string{"main.js", "sub/browser/main.js"}, nil, nil, "ambiguous"},
		{"missing lazy artifact", `<script src="main.js"></script>`, []string{"browser/main.js", "browser/lazy.js"}, []string{"main.js"}, nil, nil, "lazy.js"},
		{"relocated Windows stats", `<script src="main.js"></script>`, []string{`C:\workspace\dist\app\browser\main.js`}, []string{"main.js"}, []string{"C:/workspace/dist/app/browser/main.js"}, []string{"C:/workspace/dist/app/browser/main.js"}, ""},
		{"protocol relative URL", `<script src="//cdn.example/remote.js"></script><script src="./main.js"></script>`, []string{"main.js"}, []string{"main.js"}, []string{"main.js"}, []string{"main.js"}, ""},
		{"server cannot match basename", `<script src="main.js"></script>`, []string{"server/main.js"}, []string{"main.js"}, nil, nil, "main.js"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dist := filepath.Join(t.TempDir(), "browser")
			if err := os.MkdirAll(dist, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte(tt.html), 0600); err != nil {
				t.Fatal(err)
			}
			for _, f := range tt.files {
				p := filepath.Join(dist, filepath.FromSlash(f))
				if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			var outputs []snapshot.BundleOutput
			for _, p := range tt.paths {
				outputs = append(outputs, snapshot.BundleOutput{Path: p, Bytes: 10})
			}
			got, roots, err := BrowserOutputs(outputs, dist)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("got %v, want %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(roots, tt.wantRoots) {
				t.Fatalf("roots %v, want %v", roots, tt.wantRoots)
			}
			var paths []string
			for _, o := range got {
				paths = append(paths, o.Path)
			}
			if !reflect.DeepEqual(paths, tt.wantOutputs) {
				t.Fatalf("outputs %v, want %v", paths, tt.wantOutputs)
			}
		})
	}
}

func TestBrowserAbsolutePaths(t *testing.T) {
	dist := t.TempDir()
	for _, f := range []string{"main.js", "lazy.js"} {
		if err := os.WriteFile(filepath.Join(dist, f), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte(`<script src="main.js"></script>`), 0600); err != nil {
		t.Fatal(err)
	}
	main := filepath.ToSlash(filepath.Join(dist, "main.js"))
	got, roots, err := BrowserOutputs([]snapshot.BundleOutput{{Path: main}, {Path: filepath.ToSlash(filepath.Join(dist, "lazy.js"))}}, dist)
	if err != nil || len(got) != 2 || !reflect.DeepEqual(roots, []string{main}) {
		t.Fatalf("got %v, %v, %v", got, roots, err)
	}
	if _, _, err := BrowserOutputs(nil, t.TempDir()); err == nil || !strings.Contains(err.Error(), "index.html") {
		t.Fatalf("missing index: %v", err)
	}
}

func TestIndexCsrHtmlSupport(t *testing.T) {
	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "main.js"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "index.csr.html"), []byte(`<script src="main.js"></script>`), 0600); err != nil {
		t.Fatal(err)
	}
	got, roots, err := BrowserOutputs([]snapshot.BundleOutput{{Path: "main.js"}}, dist)
	if err != nil {
		t.Fatalf("expected index.csr.html to be parsed: %v", err)
	}
	if len(got) != 1 || len(roots) != 1 || roots[0] != "main.js" {
		t.Fatalf("unexpected outputs: got=%v roots=%v", got, roots)
	}
}
