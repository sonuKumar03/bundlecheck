package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sonuKumar03/bundleradar/internal/budget"
	"github.com/sonuKumar03/bundleradar/internal/core/diff"
	"github.com/sonuKumar03/bundleradar/internal/worktree"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
	"github.com/spf13/cobra"
)

func newDiffCommand() *cobra.Command {
	var (
		against        string
		format         string
		output         string
		driftThreshold string
		bundler        string
		buildCmd       string
		noBuild        bool
	)

	c := &cobra.Command{
		Use:   "diff [current_stats.json]",
		Short: "Compare current build against a baseline file or branch with regression attribution",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("current stats path is required")
			}
			currentPath := args[0]
			if against == "" {
				return fmt.Errorf("--against <baseline_path_or_git_ref> is required")
			}

			client := bundleradar.New()
			ctx := context.Background()

			currentBundle, err := client.Scan(ctx, bundleradar.ScanOptions{
				StatsPath: currentPath,
				Bundler:   bundler,
			})
			if err != nil {
				return fmt.Errorf("scan current bundle: %w", err)
			}

			var baseBundle *bundleradar.Bundle
			var scanErr error

			// 1. Check if against is an existing file on disk
			if fi, err := os.Stat(against); err == nil && !fi.IsDir() {
				baseBundle, scanErr = client.Scan(ctx, bundleradar.ScanOptions{
					StatsPath: against,
					Bundler:   bundler,
				})
				if scanErr != nil {
					return fmt.Errorf("scan baseline bundle %q: %w", against, scanErr)
				}
			} else {
				// 2. Fall back to git worktree resolution
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("get working directory: %w", err)
				}
				if !worktree.IsGitRepo(wd) {
					return fmt.Errorf("baseline %q is neither a file nor was a git repository detected: %w", against, err)
				}

				repoRoot, err := worktree.GetRepoRoot(wd)
				if err != nil {
					return fmt.Errorf("get repo root: %w", err)
				}

				resolvedRef, err := worktree.ResolveRef(repoRoot, against)
				if err != nil {
					return fmt.Errorf("resolve git ref %q: %w", against, err)
				}

				wtDir, cleanup, err := worktree.Create(repoRoot, resolvedRef)
				if err != nil {
					return fmt.Errorf("create worktree for %q: %w", against, err)
				}
				defer cleanup()

				if !noBuild {
					cmdToRun := buildCmd
					if cmdToRun == "" {
						cmdToRun = "npm run build"
					}
					if format != "json" {
						fmt.Fprintf(c.OutOrStdout(), "Building %q in temporary worktree (%s)...\n", resolvedRef, cmdToRun)
					}
					if err := worktree.RunBuild(wtDir, cmdToRun); err != nil {
						return fmt.Errorf("worktree build failed: %w", err)
					}
				}

				// Locate the corresponding stats file in the worktree
				targetStatsInWt := filepath.Join(wtDir, currentPath)
				if _, err := os.Stat(targetStatsInWt); err != nil {
					if !filepath.IsAbs(currentPath) {
						relPath, relErr := filepath.Rel(repoRoot, filepath.Join(wd, currentPath))
						if relErr == nil {
							targetStatsInWt = filepath.Join(wtDir, relPath)
						}
					}
				}

				baseBundle, scanErr = client.Scan(ctx, bundleradar.ScanOptions{
					StatsPath: targetStatsInWt,
					Bundler:   bundler,
				})
				if scanErr != nil {
					return fmt.Errorf("scan worktree baseline bundle (%s): %w", targetStatsInWt, scanErr)
				}
			}

			var driftBytes int64 = 1024
			if driftThreshold != "" {
				if parsed, err := budget.ParseBytes(driftThreshold); err == nil {
					driftBytes = parsed
				}
			}

			diffResult := client.Diff(baseBundle, currentBundle, diff.Options{
				DriftThreshold: driftBytes,
			})

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() { _ = cleanup() }()

			rep, err := client.Reporter(format)
			if err != nil {
				return err
			}

			return rep.Render(ctx, w, diffResult)
		},
	}

	c.Flags().StringVar(&against, "against", "", "Path to baseline stats/metafile JSON or git ref (e.g. main, HEAD~1)")
	c.Flags().StringVarP(&format, "format", "f", "terminal", "Output format: terminal, markdown, github-pr, json")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to file path")
	c.Flags().StringVar(&driftThreshold, "drift-threshold", "1KB", "Byte threshold to bucket micro-drift")
	c.Flags().StringVar(&bundler, "bundler", "", "Override bundler auto-detection")
	c.Flags().StringVar(&buildCmd, "build-cmd", "npm run build", "Build command to execute inside temporary worktree")
	c.Flags().BoolVar(&noBuild, "no-build", false, "Skip building inside temporary worktree")

	return c
}
