package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"bundlecheck/internal/graph"
)

func TestWhyCommandTextAndJSON(t *testing.T) {
	base := filepath.Join("..", "testdata", "minimal")

	// 1. JSON test on minimal fixture (has lodash)
	argsJSON := []string{
		"why", "lodash",
		"-s", filepath.Join(base, "stats.json"),
		"-d", filepath.Join(base, "browser"),
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

	if !res.Found || res.PackageName != "lodash" || res.InitialBytes != 128 {
		t.Errorf("unexpected why result: %+v", res)
	}

	// 2. Text test on minimal fixture
	out.Reset()
	errOut.Reset()
	argsText := []string{
		"why", "lodash",
		"-s", filepath.Join(base, "stats.json"),
		"-d", filepath.Join(base, "browser"),
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
	if code := Execute(args, &out, &errOut); code == 0 || !strings.Contains(errOut.String(), "required") {
		t.Fatalf("expected error on missing target, got exit %d: %s", code, errOut.String())
	}
}
