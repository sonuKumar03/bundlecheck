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
