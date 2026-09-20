package worktree_test

import (
	"os"
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/worktree"
)

func TestIsGitRepo(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	if !worktree.IsGitRepo(wd) {
		t.Errorf("expected %q to be recognized as git repo", wd)
	}

	tmpDir := t.TempDir()
	if worktree.IsGitRepo(tmpDir) {
		t.Errorf("expected %q not to be git repo", tmpDir)
	}
}

func TestGetCommitSHA(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	sha, err := worktree.GetCommitSHA(wd, "HEAD")
	if err != nil {
		t.Fatalf("unexpected error getting HEAD sha: %v", err)
	}
	if len(sha) < 4 {
		t.Errorf("expected valid short SHA, got %q", sha)
	}
}
