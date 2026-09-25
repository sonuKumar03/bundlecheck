package cmd_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/cmd"
)

func TestV2CLI_Scan(t *testing.T) {
	statsPath := filepath.Join("..", "testdata", "minimal", "stats.json")
	distDir := filepath.Join("..", "testdata", "minimal", "browser")

	var stdout, stderr bytes.Buffer
	code := cmd.Execute([]string{"scan", statsPath, "--dist", distDir, "--format", "json"}, &stdout, &stderr)
	if code != cmd.ExitCodeSuccess {
		t.Fatalf("scan failed with code %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, `"entrypoints"`) {
		t.Fatalf("scan json missing entrypoints: %s", out)
	}
}

func TestV2CLI_Diff(t *testing.T) {
	afterStats := filepath.Join("..", "testdata", "minimal", "stats.json")
	beforeStats := filepath.Join("..", "testdata", "minimal", "stats.json")

	var stdout, stderr bytes.Buffer
	code := cmd.Execute([]string{"diff", afterStats, "--against", beforeStats, "--format", "json"}, &stdout, &stderr)
	if code != cmd.ExitCodeSuccess {
		t.Fatalf("diff failed with code %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, `"entrypoints"`) {
		t.Fatalf("diff json missing entrypoints: %s", out)
	}
}

func TestV2CLI_Gate_PassAndFail(t *testing.T) {
	statsPath := filepath.Join("..", "testdata", "minimal", "stats.json")

	// 1. Pass test
	var stdout, stderr bytes.Buffer
	code := cmd.Execute([]string{"gate", statsPath, "--max-initial", "10MB"}, &stdout, &stderr)
	if code != cmd.ExitCodeSuccess {
		t.Fatalf("expected gate pass, got code %d: %s", code, stderr.String())
	}

	// 2. Fail test
	stdout.Reset()
	stderr.Reset()
	code = cmd.Execute([]string{"gate", statsPath, "--max-initial", "100B"}, &stdout, &stderr)
	if code != cmd.ExitCodePolicyViolation {
		t.Fatalf("expected gate violation code %d, got %d. Output: %s", cmd.ExitCodePolicyViolation, code, stdout.String())
	}
}

func TestV2CLI_Diff_GitWorktree(t *testing.T) {
	statsPath := filepath.Join("..", "testdata", "minimal", "stats.json")

	var stdout, stderr bytes.Buffer
	code := cmd.Execute([]string{"diff", statsPath, "--against", "HEAD", "--no-build", "--format", "json"}, &stdout, &stderr)
	if code != cmd.ExitCodeSuccess {
		t.Fatalf("diff with git ref failed with code %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, `"entrypoints"`) {
		t.Fatalf("diff json missing entrypoints: %s", out)
	}
}

func TestV2CLI_Scan_Why(t *testing.T) {
	statsPath := filepath.Join("..", "testdata", "minimal", "stats.json")

	// 1. JSON format
	var stdout, stderr bytes.Buffer
	code := cmd.Execute([]string{"scan", statsPath, "--why", "lodash", "--format", "json"}, &stdout, &stderr)
	if code != cmd.ExitCodeSuccess {
		t.Fatalf("scan --why failed with code %d: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"target": "lodash"`) {
		t.Fatalf("scan --why json missing target: %s", out)
	}

	// 2. Terminal format
	stdout.Reset()
	stderr.Reset()
	code = cmd.Execute([]string{"scan", statsPath, "--why", "lodash"}, &stdout, &stderr)
	if code != cmd.ExitCodeSuccess {
		t.Fatalf("scan --why terminal failed with code %d: %s", code, stderr.String())
	}
	out = stdout.String()
	if !strings.Contains(out, "IMPORT TRACE") {
		t.Fatalf("scan --why terminal missing header: %s", out)
	}
}

func TestV2CLI_Scan_Top(t *testing.T) {
	statsPath := filepath.Join("..", "testdata", "minimal", "stats.json")

	var stdout, stderr bytes.Buffer
	code := cmd.Execute([]string{"scan", statsPath, "--top", "2"}, &stdout, &stderr)
	if code != cmd.ExitCodeSuccess {
		t.Fatalf("scan --top failed with code %d: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "TOP CONTRIBUTING NPM PACKAGES") {
		t.Fatalf("scan --top output missing top packages table: %s", out)
	}
}
