package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sonuKumar03/bundleradar/internal/worktree"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

// resolveBaselineBundle resolves and scans a baseline bundle from either a local file or a git ref.
func resolveBaselineBundle(
	ctx context.Context,
	client *bundleradar.Client,
	against string,
	currentStatsPath string,
	bundler string,
	buildCmd string,
	noBuild bool,
	out io.Writer,
	format string,
) (*bundleradar.Bundle, func(), error) {
	noopCleanup := func() {}

	// 1. Check if against is an existing file on disk
	if fi, err := os.Stat(against); err == nil && !fi.IsDir() {
		baseBundle, err := client.Scan(ctx, bundleradar.ScanOptions{
			StatsPath: against,
			Bundler:   bundler,
		})
		if err != nil {
			return nil, noopCleanup, fmt.Errorf("scan baseline bundle %q: %w", against, err)
		}
		return baseBundle, noopCleanup, nil
	}

	// 2. Fall back to git worktree resolution
	wd, err := os.Getwd()
	if err != nil {
		return nil, noopCleanup, fmt.Errorf("get working directory: %w", err)
	}
	if !worktree.IsGitRepo(wd) {
		return nil, noopCleanup, fmt.Errorf("baseline %q is neither a file nor was a git repository detected: %w", against, err)
	}

	repoRoot, err := worktree.GetRepoRoot(wd)
	if err != nil {
		return nil, noopCleanup, fmt.Errorf("get repo root: %w", err)
	}

	resolvedRef, err := worktree.ResolveRef(repoRoot, against)
	if err != nil {
		return nil, noopCleanup, fmt.Errorf("resolve git ref %q: %w", against, err)
	}

	wtDir, cleanup, err := worktree.Create(repoRoot, resolvedRef)
	if err != nil {
		return nil, noopCleanup, fmt.Errorf("create worktree for %q: %w", against, err)
	}

	if !noBuild {
		cmdToRun := buildCmd
		if cmdToRun == "" {
			cmdToRun = "npm run build"
		}
		if format != "json" && out != nil {
			fmt.Fprintf(out, "Building %q in temporary worktree (%s)...\n", resolvedRef, cmdToRun)
		}
		if err := worktree.RunBuild(wtDir, cmdToRun); err != nil {
			cleanup()
			return nil, noopCleanup, fmt.Errorf("worktree build failed: %w", err)
		}
	}

	// Locate the corresponding stats file in the worktree
	targetStatsInWt := filepath.Join(wtDir, currentStatsPath)
	if _, err := os.Stat(targetStatsInWt); err != nil {
		if !filepath.IsAbs(currentStatsPath) {
			relPath, relErr := filepath.Rel(repoRoot, filepath.Join(wd, currentStatsPath))
			if relErr == nil {
				targetStatsInWt = filepath.Join(wtDir, relPath)
			}
		}
	}

	baseBundle, err := client.Scan(ctx, bundleradar.ScanOptions{
		StatsPath: targetStatsInWt,
		Bundler:   bundler,
	})
	if err != nil {
		cleanup()
		return nil, noopCleanup, fmt.Errorf("scan worktree baseline bundle (%s): %w", targetStatsInWt, err)
	}

	return baseBundle, cleanup, nil
}
