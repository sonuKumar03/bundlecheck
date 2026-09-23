package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/analysis"
	"github.com/sonuKumar03/bundleradar/internal/graph"
)

func ensureAngularEsbuildBuilt(t *testing.T, base string) {
	t.Helper()
	statsPath := filepath.Join(base, "dist", "bundleradar-smoke", "stats.json")
	if _, err := os.Stat(statsPath); err == nil {
		return
	}

	ngBin := filepath.Join(base, "node_modules", ".bin", "ng")
	if _, err := os.Stat(ngBin); os.IsNotExist(err) {
		t.Skip("testdata/angular-esbuild dependencies not installed")
	}

	cmd := exec.Command(ngBin, "build", "--stats-json")
	cmd.Dir = base
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ng build failed or could not run in current environment: %v\nOutput: %s", err, string(out))
	}
}

func TestAngularEsbuildProject(t *testing.T) {
	base := filepath.Join("..", "testdata", "angular-esbuild")
	if _, err := os.Stat(base); os.IsNotExist(err) {
		t.Skip("testdata/angular-esbuild directory not present")
	}
	ensureAngularEsbuildBuilt(t, base)

	t.Run("Summary auto-discovery and invariant checks", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Execute([]string{"summary", base, "-f", "json"}, &out, &errOut)
		if code != 0 || errOut.Len() != 0 {
			t.Fatalf("summary exit %d, stderr: %s", code, errOut.String())
		}

		var res analysis.AnalysisResult
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			t.Fatalf("failed to parse summary JSON: %v", err)
		}

		if res.Summary.InitialJS <= 0 {
			t.Errorf("expected InitialJS > 0, got %d", res.Summary.InitialJS)
		}
		if res.Summary.TotalJS < res.Summary.InitialJS {
			t.Errorf("expected TotalJS >= InitialJS, got TotalJS=%d, InitialJS=%d", res.Summary.TotalJS, res.Summary.InitialJS)
		}

		pkgMap := make(map[string]bool)
		for _, p := range res.Packages {
			pkgMap[p.Name] = true
			if p.TotalBytes <= 0 {
				t.Errorf("package %s has non-positive TotalBytes: %d", p.Name, p.TotalBytes)
			}
		}

		expectedPackages := []string{"@angular/core", "@angular/router", "rxjs", "date-fns"}
		for _, pkg := range expectedPackages {
			if _, ok := pkgMap[pkg]; !ok {
				t.Errorf("expected package %s in summary, got: %v", pkg, pkgMap)
			}
		}
	})

	t.Run("Why traces package without hardcoded chunk names", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Execute([]string{"why", base, "date-fns", "-f", "json"}, &out, &errOut)
		if code != 0 || errOut.Len() != 0 {
			t.Fatalf("why exit %d, stderr: %s", code, errOut.String())
		}

		var res graph.WhyResult
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			t.Fatalf("failed to parse why JSON: %v", err)
		}

		if !res.Found || res.PackageName != "date-fns" {
			t.Fatalf("expected date-fns to be found, got: %+v", res)
		}
		if res.TotalBytes <= 0 {
			t.Errorf("expected TotalBytes > 0, got %d", res.TotalBytes)
		}
		if len(res.Chains) == 0 {
			t.Fatal("expected import chains for date-fns, got 0")
		}

		foundLazySource := false
		for _, chain := range res.Chains {
			if chain.Output == "" {
				t.Errorf("expected non-empty output chunk for chain: %+v", chain)
			}
			for _, step := range chain.Path {
				if strings.Contains(step, "src/app/lazy.ts") {
					foundLazySource = true
				}
			}
		}
		if !foundLazySource {
			t.Errorf("expected at least one chain to originate from src/app/lazy.ts")
		}
	})

	t.Run("Inspect dynamic chunk without hardcoded names", func(t *testing.T) {
		browserDir := filepath.Join(base, "dist", "bundleradar-smoke", "browser")
		targetChunk := findAnyJSChunk(t, filepath.Join(base, "dist", "bundleradar-smoke"))

		var out, errOut bytes.Buffer
		code := Execute([]string{"inspect", targetChunk, "-s", base, "-f", "json"}, &out, &errOut)
		if code != 0 || errOut.Len() != 0 {
			t.Fatalf("inspect exit %d, stderr: %s", code, errOut.String())
		}

		var res analysis.InspectResult
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			t.Fatalf("failed to parse inspect JSON: %v", err)
		}

		if res.Bytes <= 0 {
			t.Errorf("expected chunk bytes > 0, got %d in %s", res.Bytes, targetChunk)
		}
		if len(res.Packages) == 0 {
			t.Errorf("expected packages in chunk %s (dir: %s)", targetChunk, browserDir)
		}
		for _, p := range res.Packages {
			if p.Name == "" || p.Bytes <= 0 {
				t.Errorf("invalid package contributor: %+v", p)
			}
		}
	})

	t.Run("Suggest succeeds on live Angular build", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Execute([]string{"suggest", base, "-f", "json"}, &out, &errOut)
		if code != 0 || errOut.Len() != 0 {
			t.Fatalf("suggest exit %d, stderr: %s", code, errOut.String())
		}
	})

	t.Run("Check enforces budgets dynamically", func(t *testing.T) {
		// Generous budget should pass
		var out, errOut bytes.Buffer
		code := Execute([]string{"check", base, "--max-initial", "100MB"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("check failed with generous budget: %s", errOut.String())
		}

		// 1-byte budget should fail
		out.Reset()
		errOut.Reset()
		code = Execute([]string{"check", base, "--max-initial", "1B"}, &out, &errOut)
		if code == 0 {
			t.Fatal("expected check to fail with 1B budget")
		}
	})
}
