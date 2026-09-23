package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/graph"
)

func TestWhyCommandTextAndJSON(t *testing.T) {
	base := filepath.Join("..", "testdata", "minimal")

	// 1. JSON test on minimal fixture (has lodash)
	argsJSON := []string{
		"why", "lodash",
		"-s", base,
		"-f", "json",
	}

	var out, errOut bytes.Buffer
	if code := Execute(argsJSON, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	var res graph.WhyResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}

	if !res.Found || res.PackageName != "lodash" || res.InitialBytes <= 0 {
		t.Errorf("unexpected why result: %+v", res)
	}

	// 2. Text test on minimal fixture
	out.Reset()
	errOut.Reset()
	argsText := []string{
		"why", "lodash",
		"-s", base,
	}

	if code := Execute(argsText, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	if !strings.Contains(out.String(), "Dependency Trace for \"lodash\"") || !strings.Contains(out.String(), "INITIAL") {
		t.Errorf("unexpected text output: %s", out.String())
	}
}

func TestWhyCommandMissingTarget(t *testing.T) {
	args := []string{"why"}
	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code == 0 || (!strings.Contains(errOut.String(), "accepts between 1 and 2 arg") && !strings.Contains(errOut.String(), "required")) {
		t.Fatalf("expected error on missing target, got exit %d: %s", code, errOut.String())
	}
}

func TestWhyCommandPositionalStats(t *testing.T) {
	base := filepath.Join("..", "testdata", "minimal")
	statsFile := filepath.Join(base, "stats.json")

	args := []string{
		"why",
		statsFile,
		"lodash",
		"-d", filepath.Join(base, "browser"),
		"-f", "json",
	}

	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	var res graph.WhyResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
	if !res.Found {
		t.Error("expected package lodash to be found via positional stats")
	}
}

func TestWhyPackageFlagWithoutPositionalTarget(t *testing.T) {
	base := filepath.Join("..", "testdata", "minimal")
	var out, errOut bytes.Buffer
	code := Execute([]string{"why", "--package", "lodash", "-s", base, "-f", "json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("package flag rejected: %s", errOut.String())
	}
	var result graph.WhyResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || !result.Found || result.InitialBytes <= 0 {
		t.Fatalf("unexpected trace: result=%+v err=%v", result, err)
	}
}
