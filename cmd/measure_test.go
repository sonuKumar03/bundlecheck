package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/comparison"
)

func TestMeasureCommandSuccess(t *testing.T) {
	baselinePath := filepath.Join("..", "testdata", "comparison", "before.json")
	afterBase := filepath.Join("..", "testdata", "lazy-import")

	args := []string{
		"measure",
		"-s", afterBase,
		"-b", baselinePath,
		"-f", "json",
	}

	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	var res comparison.Result
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("stdout not valid json: %v", err)
	}
	if res.Summary.After.InitialJS <= 0 {
		t.Errorf("expected after InitialJS > 0, got %d", res.Summary.After.InitialJS)
	}
}

func TestMeasureRegressionBudget(t *testing.T) {
	baselinePath := filepath.Join("..", "testdata", "comparison", "before.json")
	afterBase := filepath.Join("..", "testdata", "lazy-import")

	// Expect fail: max initial delta set to 0, but lazy-import initial JS is much bigger than before.json (100 vs 163840)
	args := []string{
		"measure",
		"-s", afterBase,
		"-b", baselinePath,
		"--max-initial-delta", "0B",
	}

	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code == 0 {
		t.Fatal("expected failure on regression limit breached")
	}
}

func TestMeasureMarkdown(t *testing.T) {
	baselinePath := filepath.Join("..", "testdata", "comparison", "before.json")
	afterBase := filepath.Join("..", "testdata", "lazy-import")

	args := []string{
		"measure",
		"-s", afterBase,
		"-b", baselinePath,
		"-f", "markdown",
	}

	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	output := out.String()
	if !strings.Contains(output, "## 📊 Angular Bundle Comparison") {
		t.Errorf("expected markdown header in measure output, got: %s", output)
	}
}

func TestMeasureNamedBaselineResolution(t *testing.T) {
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpWd := t.TempDir()
	if err := os.Chdir(tmpWd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	base := filepath.Join(origWd, "..", "testdata", "lazy-import")
	absBase, _ := filepath.Abs(base)

	// Save baseline as 'feature-base'
	var out, errOut bytes.Buffer
	code := Execute([]string{
		"baseline", "save",
		"--name", "feature-base",
		"-s", absBase,
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("save baseline failed: %s", errOut.String())
	}

	// Run measure referencing the name directly
	out.Reset()
	errOut.Reset()
	code = Execute([]string{
		"measure",
		"-b", "feature-base",
		"-s", absBase,
		"-f", "json",
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("measure with named baseline failed: %s", errOut.String())
	}

	var res comparison.Result
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if res.Summary.Delta.InitialJS != 0 {
		t.Errorf("expected 0 delta comparing same build, got %d", res.Summary.Delta.InitialJS)
	}
}

func TestMeasureGitHubPR(t *testing.T) {
	baselinePath := filepath.Join("..", "testdata", "comparison", "before.json")
	afterBase := filepath.Join("..", "testdata", "lazy-import")

	args := []string{
		"measure",
		"-s", afterBase,
		"-b", baselinePath,
		"-f", "github-pr",
	}

	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	output := out.String()
	if !strings.Contains(output, "<!-- bundlecheck-comment -->") {
		t.Errorf("expected sticky comment marker in measure output, got: %s", output)
	}
	if !strings.Contains(output, "## 📦 BundleCheck PR Report") {
		t.Errorf("expected PR report header in measure output, got: %s", output)
	}
	if !strings.Contains(output, "<code>[") {
		t.Errorf("expected visual diff bar in measure output, got: %s", output)
	}
}


