package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bundlecheck/internal/analysis"
)

func TestNxWorkspaceCommands(t *testing.T) {
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	nxRoot := filepath.Join(origWd, "..", "testdata", "nx-workspace")
	if _, err := os.Stat(nxRoot); os.IsNotExist(err) {
		t.Skip("testdata/nx-workspace fixture not present")
	}
	if _, err := os.Stat(filepath.Join(nxRoot, "dist")); os.IsNotExist(err) {
		t.Skip("testdata/nx-workspace/dist build artifacts not present")
	}

	if err := os.Chdir(nxRoot); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(origWd)
	}()

	t.Run("Summary without project flag fails gracefully", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Execute([]string{"summary"}, &out, &errOut)
		if code == 0 {
			t.Fatal("expected failure when running in multi-app Nx workspace without --project")
		}
		if !strings.Contains(errOut.String(), "multiple Angular build outputs found") {
			t.Fatalf("expected multiple build outputs error, got: %s", errOut.String())
		}
	})

	t.Run("Summary portal in Nx workspace", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Execute([]string{"summary", "--project", "portal", "--format", "json"}, &out, &errOut)
		if code != 0 || errOut.Len() != 0 {
			t.Fatalf("exit %d, stderr: %s", code, errOut.String())
		}

		var res analysis.AnalysisResult
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			t.Fatalf("failed to parse JSON: %v\nOutput: %s", err, out.String())
		}

		if res.Summary.InitialJS == 0 || res.Summary.TotalJS == 0 {
			t.Errorf("expected non-zero InitialJS and TotalJS in portal summary, got %+v", res.Summary)
		}

		// Verify heavy dependencies are present in packages list
		foundMoment := false
		foundChartJs := false
		foundPdfJs := false
		foundLodash := false
		for _, pkg := range res.Packages {
			switch pkg.Name {
			case "moment":
				foundMoment = true
			case "chart.js":
				foundChartJs = true
			case "pdfjs-dist":
				foundPdfJs = true
			case "lodash":
				foundLodash = true
			}
		}

		if !foundMoment || !foundChartJs || !foundPdfJs || !foundLodash {
			t.Errorf("missing expected packages in portal: moment=%v, chart.js=%v, pdfjs-dist=%v, lodash=%v",
				foundMoment, foundChartJs, foundPdfJs, foundLodash)
		}
	})

	t.Run("Summary admin-dashboard in Nx workspace", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Execute([]string{"summary", "--project", "admin-dashboard", "--format", "json"}, &out, &errOut)
		if code != 0 || errOut.Len() != 0 {
			t.Fatalf("exit %d, stderr: %s", code, errOut.String())
		}

		var res analysis.AnalysisResult
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		foundExcel := false
		for _, pkg := range res.Packages {
			if pkg.Name == "exceljs" {
				foundExcel = true
			}
		}
		if !foundExcel {
			t.Errorf("expected exceljs in admin-dashboard packages")
		}
	})

	t.Run("Suggest on portal detects optimization opportunities", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Execute([]string{"suggest", "--project", "portal"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("exit %d, stderr: %s", code, errOut.String())
		}
		output := out.String()
		// Should advise on moment replacement and lodash
		if !strings.Contains(output, "moment") && !strings.Contains(output, "date-fns") {
			t.Errorf("expected date/moment suggestion in output: %s", output)
		}
	})

	t.Run("Why command traces package in Nx workspace", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Execute([]string{"why", "moment", "--project", "portal"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("exit %d, stderr: %s", code, errOut.String())
		}
		output := out.String()
		if !strings.Contains(output, "moment") {
			t.Errorf("expected why output to mention moment, got: %s", output)
		}
	})
}
