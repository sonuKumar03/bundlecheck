// Package worktree manages temporary git worktrees for isolated branch builds.
package worktree

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// IsGitRepo checks if the specified directory is inside a git repository.
func IsGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// GetRepoRoot returns the top-level directory of the git repository.
func GetRepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("determine git repository root: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ResolveRef resolves a git ref in repoRoot, checking if ref exists directly or
// falling back to origin/<ref> if the ref is only available remotely (common in CI PR checkouts).
func ResolveRef(dir string, ref string) (string, error) {
	if !IsGitRepo(dir) {
		return "", fmt.Errorf("directory %q is not a git repository", dir)
	}
	repoRoot, err := GetRepoRoot(dir)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("git", "rev-parse", "--verify", ref)
	cmd.Dir = repoRoot
	if err := cmd.Run(); err == nil {
		return ref, nil
	}

	if !strings.HasPrefix(ref, "origin/") {
		originRef := "origin/" + ref
		cmdOrigin := exec.Command("git", "rev-parse", "--verify", originRef)
		cmdOrigin.Dir = repoRoot
		if err := cmdOrigin.Run(); err == nil {
			return originRef, nil
		}
	}

	return ref, nil
}

// GetCommitSHA resolves a git ref (branch, tag, commit) to a short commit SHA.
func GetCommitSHA(dir string, ref string) (string, error) {
	resolvedRef, _ := ResolveRef(dir, ref)
	if resolvedRef == "" {
		resolvedRef = ref
	}
	cmd := exec.Command("git", "rev-parse", "--short", resolvedRef)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve git ref %q: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Create creates a temporary git worktree checked out to the given ref.
// It returns the worktree directory path, a cleanup function, and any error encountered.
func Create(rootDir string, ref string) (string, func(), error) {
	if !IsGitRepo(rootDir) {
		return "", nil, fmt.Errorf("directory %q is not a git repository", rootDir)
	}

	repoRoot, err := GetRepoRoot(rootDir)
	if err != nil {
		return "", nil, err
	}

	tempDir, err := os.MkdirTemp("", "bundlecheck-wt-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp worktree dir: %w", err)
	}

	resolvedRef, _ := ResolveRef(repoRoot, ref)
	if resolvedRef == "" {
		resolvedRef = ref
	}

	// Add git worktree
	cmd := exec.Command("git", "worktree", "add", "--detach", tempDir, resolvedRef)
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("git worktree add failed for ref %q: %s", ref, strings.TrimSpace(stderr.String()))
	}

	// Symlink node_modules from repoRoot if present to avoid npm install in worktree
	srcNodeModules := filepath.Join(repoRoot, "node_modules")
	destNodeModules := filepath.Join(tempDir, "node_modules")
	if fi, err := os.Stat(srcNodeModules); err == nil && fi.IsDir() {
		if _, err := os.Stat(destNodeModules); os.IsNotExist(err) {
			_ = os.Symlink(srcNodeModules, destNodeModules)
		}
	}

	cleanup := func() {
		rmCmd := exec.Command("git", "worktree", "remove", "--force", tempDir)
		rmCmd.Dir = repoRoot
		_ = rmCmd.Run()
		_ = os.RemoveAll(tempDir)
	}

	return tempDir, cleanup, nil
}

// RunBuild executes the build command within the specified worktree directory.
func RunBuild(worktreeDir string, buildCmd string) error {
	if strings.TrimSpace(buildCmd) == "" {
		pkgJSON := filepath.Join(worktreeDir, "package.json")
		if _, err := os.Stat(pkgJSON); err == nil {
			buildCmd = "npm run build"
		} else {
			return fmt.Errorf("no build command provided and package.json not found in %q", worktreeDir)
		}
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/c", buildCmd)
	} else {
		cmd = exec.Command("sh", "-c", buildCmd)
	}

	cmd.Dir = worktreeDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errOutput := strings.TrimSpace(stderr.String())
		if errOutput == "" {
			errOutput = strings.TrimSpace(stdout.String())
		}
		return fmt.Errorf("build command %q failed:\n%s", buildCmd, errOutput)
	}

	return nil
}
