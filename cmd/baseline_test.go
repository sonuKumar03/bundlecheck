package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/baseline"
)

func TestBaselineCommand(t *testing.T) {
	base := filepath.Join("..", "testdata", "minimal")
	targetFile := filepath.Join(t.TempDir(), "baseline.json")

	args := []string{
		"baseline",
		"-s", base,
		"-o", targetFile,
		"-f", "json",
	}

	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read written baseline: %v", err)
	}

	var saved analysis.AnalysisResult
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("invalid json in saved baseline: %v", err)
	}
	if saved.Summary.InitialJS <= 0 {
		t.Errorf("expected InitialJS > 0, got %d", saved.Summary.InitialJS)
	}
}

func TestBaselineLifecycleSubcommands(t *testing.T) {
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpWd := t.TempDir()
	if err := os.Chdir(tmpWd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	base := filepath.Join(origWd, "..", "testdata", "minimal")
	absBase, err := filepath.Abs(base)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	// 1. Save named baseline 'v1'
	var out, errOut bytes.Buffer
	code := Execute([]string{
		"baseline", "save",
		"--name", "v1",
		"-s", absBase,
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("baseline save failed: %s", errOut.String())
	}
	if !strings.Contains(out.String(), "Successfully captured and saved baseline") {
		t.Errorf("unexpected save output: %s", out.String())
	}

	// 2. List baselines
	out.Reset()
	errOut.Reset()
	code = Execute([]string{"baseline", "list"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("baseline list failed: %s", errOut.String())
	}
	if !strings.Contains(out.String(), "v1") || !strings.Contains(out.String(), "active") {
		t.Errorf("expected 'v1' and 'active' in list output: %s", out.String())
	}

	// 3. List in JSON format
	out.Reset()
	errOut.Reset()
	code = Execute([]string{"baseline", "list", "-f", "json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("baseline list json failed: %s", errOut.String())
	}
	var listData struct {
		Active    string                  `json:"active"`
		Baselines []baseline.BaselineInfo `json:"baselines"`
	}
	if err := json.Unmarshal(out.Bytes(), &listData); err != nil {
		t.Fatalf("invalid json from baseline list: %v", err)
	}
	if listData.Active != "v1" || len(listData.Baselines) != 1 {
		t.Errorf("unexpected list json data: %+v", listData)
	}

	// 4. Show baseline
	out.Reset()
	errOut.Reset()
	code = Execute([]string{"baseline", "show", "v1"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("baseline show failed: %s", errOut.String())
	}
	if !strings.Contains(out.String(), "Baseline Snapshot Details") || !strings.Contains(out.String(), "Name:       v1") {
		t.Errorf("unexpected show output: %s", out.String())
	}

	// 5. Use baseline
	out.Reset()
	errOut.Reset()
	code = Execute([]string{"baseline", "use", "v1"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("baseline use failed: %s", errOut.String())
	}
	if !strings.Contains(out.String(), "Switched active baseline to \"v1\"") {
		t.Errorf("unexpected use output: %s", out.String())
	}

	// 6. Delete baseline
	out.Reset()
	errOut.Reset()
	code = Execute([]string{"baseline", "delete", "v1"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("baseline delete failed: %s", errOut.String())
	}
	if !strings.Contains(out.String(), "Deleted baseline snapshot \"v1\"") {
		t.Errorf("unexpected delete output: %s", out.String())
	}
}

func TestBaselineCreateSubcommand(t *testing.T) {
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpWd := t.TempDir()
	if err := os.Chdir(tmpWd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	base := filepath.Join(origWd, "..", "testdata", "minimal")
	absBase, err := filepath.Abs(base)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	var out, errOut bytes.Buffer
	code := Execute([]string{
		"baseline", "create", "my-base",
		"-s", absBase,
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("baseline create failed: %s", errOut.String())
	}
	if !strings.Contains(out.String(), "Successfully captured and saved baseline") {
		t.Errorf("unexpected create output: %s", out.String())
	}

	out.Reset()
	errOut.Reset()
	code = Execute([]string{"baseline", "show", "my-base"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("baseline show failed: %s", errOut.String())
	}
	if !strings.Contains(out.String(), "Name:       my-base") {
		t.Errorf("expected Name: my-base in output: %s", out.String())
	}
}

func TestBaselineFromGitWorktree(t *testing.T) {
	defer func() {
		_ = baseline.Delete(baseline.DefaultDir, "test-git-base")
	}()

	var out, errOut bytes.Buffer
	code := Execute([]string{
		"baseline", "create", "test-git-base",
		"--from-git", "HEAD",
		"--no-build",
		"-s", "testdata/minimal",
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("baseline create --from-git failed: %s", errOut.String())
	}

	snapshotFile, err := baseline.LoadSnapshot("test-git-base")
	if err != nil {
		t.Fatalf("load snapshot failed: %v", err)
	}
	if snapshotFile.Metadata == nil || snapshotFile.Metadata.GitRef != "HEAD" {
		t.Errorf("expected GitRef 'HEAD', got %+v", snapshotFile.Metadata)
	}
	if snapshotFile.Metadata.CommitSHA == "" {
		t.Errorf("expected CommitSHA to be recorded")
	}
}
