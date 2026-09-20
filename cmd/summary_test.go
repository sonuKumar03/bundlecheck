package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/analysis"
)

func TestSummaryFixtures(t *testing.T) {
	for _, tt := range []struct {
		name             string
		expectedPackages []string
	}{
		{"minimal", []string{"lodash"}},
		{"lazy-import", []string{"pdfjs-dist", "@angular/core", "rxjs", "date-fns"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			base := filepath.Join("..", "testdata", tt.name)
			args := []string{"summary", base, "--format", "json"}
			var previous string
			for i := 0; i < 5; i++ {
				var out, errOut bytes.Buffer
				if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
					t.Fatalf("exit %d: %s", code, errOut.String())
				}
				var got analysis.AnalysisResult
				if err := json.Unmarshal(out.Bytes(), &got); err != nil {
					t.Fatalf("stdout not clean JSON: %s", out.String())
				}
				if got.Summary.InitialJS <= 0 || got.Summary.TotalJS < got.Summary.InitialJS {
					t.Fatalf("invalid summary totals: %+v", got.Summary)
				}
				pkgMap := make(map[string]bool)
				for _, p := range got.Packages {
					pkgMap[p.Name] = true
					if p.TotalBytes <= 0 {
						t.Errorf("package %s has non-positive TotalBytes: %d", p.Name, p.TotalBytes)
					}
				}
				for _, wantPkg := range tt.expectedPackages {
					if !pkgMap[wantPkg] {
						t.Errorf("expected package %s in summary, got packages: %v", wantPkg, pkgMap)
					}
				}
				if i > 0 && previous != out.String() {
					t.Fatal("nondeterministic JSON")
				}
				previous = out.String()
			}
			var out, errOut bytes.Buffer
			if Execute(args[:len(args)-2], &out, &errOut) != 0 || !strings.HasPrefix(out.String(), "Angular Bundle Summary\n") {
				t.Fatalf("default text: %s, %s", out.String(), errOut.String())
			}
		})
	}
}

func TestSummaryFileOutputAndOptions(t *testing.T) {
	base := filepath.Join("..", "testdata", "lazy-import")
	outPath := filepath.Join(t.TempDir(), "summary.json")

	args := []string{"summary", base, "-o", outPath, "-f", "json"}
	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("expected stdout empty when -o used, got %s", out.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var res analysis.AnalysisResult
	if err := json.Unmarshal(data, &res); err != nil {
		t.Fatal(err)
	}
	if res.Summary.InitialJS <= 0 {
		t.Errorf("expected InitialJS > 0, got %d", res.Summary.InitialJS)
	}

	// Test text options: filter and top
	out.Reset()
	errOut.Reset()
	textArgs := []string{"summary", base, "--filter", "angular", "--top", "5"}
	if code := Execute(textArgs, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "@angular/core") || strings.Contains(out.String(), "pdfjs-dist") {
		t.Errorf("filter mismatch in text output: %s", out.String())
	}
}

func TestSummaryMarkdown(t *testing.T) {
	base := filepath.Join("..", "testdata", "lazy-import")
	args := []string{"summary", base, "-f", "markdown", "--suggest", "--gzip"}
	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	output := out.String()
	if !strings.Contains(output, "## 📦 Angular Bundle Summary") {
		t.Errorf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "| Category | Size | Gzip |") {
		t.Errorf("expected markdown summary table with gzip, got: %s", output)
	}
	if !strings.Contains(output, "### Top NPM Contributors") {
		t.Errorf("expected top npm contributors section, got: %s", output)
	}
	if !strings.Contains(output, "## 🔍 Initial Bundle Culprits & Contributors") {
		t.Errorf("expected suggestions section in markdown, got: %s", output)
	}
}

func TestSummaryErrors(t *testing.T) {
	base := filepath.Join("..", "testdata", "minimal")
	valid := []string{"summary", "--stats", filepath.Join(base, "stats.json"), "--dist", filepath.Join(base, "browser")}
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"missing flags with no artifacts", []string{"summary"}, "no Angular build artifacts found"},
		{"missing dist", []string{"summary", "--stats", "stats.json"}, "missing --dist"},
		{"invalid format", append(append([]string{}, valid...), "--format", "yaml"), "format"},
		{"missing stats", []string{"summary", "--stats", "missing.json", "--dist", filepath.Join(base, "browser"), "--format", "json"}, "stats"},
		{"missing index", []string{"summary", "--stats", filepath.Join(base, "stats.json"), "--dist", t.TempDir(), "--format", "json"}, "index.html"},
		{"too many positional args", append(append([]string{}, valid...), "extra1", "extra2", "extra3"), "accepts at most 2 arg(s)"},
		{"unknown command", []string{"nonexistent"}, "unknown command"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if Execute(tt.args, &out, &errOut) == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), tt.want) {
				t.Fatalf("stdout %q, stderr %q", out.String(), errOut.String())
			}
		})
	}
	p := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(p, []byte(`{"inputs":`), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if Execute([]string{"summary", "--stats", p, "--dist", filepath.Join(base, "browser"), "--format", "json"}, &out, &errOut) == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), "JSON") {
		t.Fatalf("malformed stats: %q, %q", out.String(), errOut.String())
	}
}

func TestSummaryPositionalArguments(t *testing.T) {
	base := filepath.Join("..", "testdata", "minimal")
	statsFile := filepath.Join(base, "stats.json")
	distDir := filepath.Join(base, "browser")

	var out, errOut bytes.Buffer
	code := Execute([]string{"summary", statsFile, distDir, "-f", "json"}, &out, &errOut)
	if code != 0 || errOut.Len() != 0 {
		t.Fatalf("summary positional args failed: %s", errOut.String())
	}

	var res analysis.AnalysisResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if res.Summary.InitialJS <= 0 {
		t.Errorf("expected InitialJS > 0, got %d", res.Summary.InitialJS)
	}
}

func TestSummaryPositionalProject(t *testing.T) {
	nxRoot := filepath.Join("..", "testdata", "nx-workspace")
	if _, err := os.Stat(filepath.Join(nxRoot, "dist", "apps", "portal", "stats.json")); os.IsNotExist(err) {
		if os.Getenv("BUNDLECHECK_REQUIRE_E2E") != "" {
			t.Fatalf("required E2E artifact missing: %s/dist/apps/portal/stats.json", nxRoot)
		}
		t.Skip("portal build artifacts not present")
	}

	var out, errOut bytes.Buffer
	code := Execute([]string{"summary", nxRoot, "portal", "-f", "json"}, &out, &errOut)
	if code != 0 || errOut.Len() != 0 {
		t.Fatalf("summary positional project failed: %s", errOut.String())
	}

	var res analysis.AnalysisResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if res.Summary.InitialJS <= 0 {
		t.Errorf("expected InitialJS > 0, got %d", res.Summary.InitialJS)
	}
}

func TestSummaryDirectStatsPathInMultiOutputWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	minStats, err := os.ReadFile(filepath.Join("..", "testdata", "minimal", "stats.json"))
	if err != nil {
		t.Fatal(err)
	}
	minHTML, err := os.ReadFile(filepath.Join("..", "testdata", "minimal", "browser", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	minJS, err := os.ReadFile(filepath.Join("..", "testdata", "minimal", "browser", "main.js"))
	if err != nil {
		t.Fatal(err)
	}

	// Create two distinct app build outputs
	for _, app := range []string{"admin", "portal"} {
		appDist := filepath.Join(tmpDir, "dist", "apps", app)
		if err := os.MkdirAll(filepath.Join(appDist, "browser"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(appDist, "stats.json"), minStats, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(appDist, "browser", "index.html"), minHTML, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(appDist, "browser", "main.js"), minJS, 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 1. Calling summary on root without flags fails with multiple candidates error
	var outMulti, errMulti bytes.Buffer
	codeMulti := Execute([]string{"summary", tmpDir}, &outMulti, &errMulti)
	if codeMulti == 0 || !strings.Contains(errMulti.String(), "multiple Angular build outputs") {
		t.Fatalf("expected multiple candidates error, got code %d: %s", codeMulti, errMulti.String())
	}

	// 2. Calling summary with direct stats.json path succeeds deterministically by selecting sibling browser directory
	portalStats := filepath.Join(tmpDir, "dist", "apps", "portal", "stats.json")
	var out, errOut bytes.Buffer
	code := Execute([]string{"summary", portalStats, "-f", "json"}, &out, &errOut)
	if code != 0 || errOut.Len() != 0 {
		t.Fatalf("expected direct stats.json to succeed deterministically, got exit %d: %s", code, errOut.String())
	}

	var res analysis.AnalysisResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if res.Summary.InitialJS <= 0 {
		t.Errorf("expected InitialJS > 0, got %d", res.Summary.InitialJS)
	}
}

func TestSummaryRealOutputContract(t *testing.T) {
	base := filepath.Join("..", "testdata", "minimal")
	var stdout, stderr bytes.Buffer
	code := Execute([]string{"summary", base, "--gzip", "--suggest"}, &stdout, &stderr)
	if code != ExitCodeSuccess || stderr.Len() != 0 {
		t.Fatalf("summary command failed: code %d, stderr: %s", code, stderr.String())
	}

	out := stdout.String()
	// Real fixture structure assertions:
	if !strings.Contains(out, "Angular Bundle Summary") {
		t.Errorf("missing summary title in output:\n%s", out)
	}
	if !strings.Contains(out, "Initial JS") || !strings.Contains(out, "Total JS") {
		t.Errorf("missing JS totals in output:\n%s", out)
	}
	if !strings.Contains(out, "Largest initial packages") {
		t.Errorf("missing packages section in output:\n%s", out)
	}
	if !strings.Contains(out, "lodash") {
		t.Errorf("missing contributing package lodash in output:\n%s", out)
	}
}

