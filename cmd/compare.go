package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundleradar/internal/analysis"
	"github.com/sonuKumar03/bundleradar/internal/baseline"
	"github.com/sonuKumar03/bundleradar/internal/budget"
	"github.com/sonuKumar03/bundleradar/internal/comparison"
	"github.com/sonuKumar03/bundleradar/internal/report"
	"github.com/sonuKumar03/bundleradar/internal/snapshot"
)

func compareCommand() *cobra.Command {
	var (
		before string
		after  string
		format string
		output string
		filter         string
		top            int
		all            bool
		project        string
		driftThreshold string
	)

	c := &cobra.Command{
		Use:   "compare [before.json] <after.json|stats.json>",
		Short: "Compare two saved summary snapshots or a baseline against a build",
		Long: `Compare two bundle snapshots (or an active baseline against a build) and calculate exact byte and percentage deltas.
Supports both saved summary JSON files and raw Angular/esbuild build directories or stats.json files.`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 2 {
				before = args[0]
				after = args[1]
			} else if len(args) == 1 {
				if before == "" {
					after = args[0]
				} else {
					after = args[0]
				}
			}

			if before == "" {
				// Try default active baseline if exists
				def := baseline.ResolvePath("")
				if fi, err := os.Stat(def); err == nil && !fi.IsDir() {
					before = def
				}
			}

			if before == "" || after == "" {
				return fmt.Errorf("both before and after paths are required (e.g. 'bundleradar compare baseline.json current.json' or -b and -a)")
			}
			if format != "text" && format != "json" && !report.IsMarkdownFormat(format) && !report.IsGitHubPRFormat(format) {
				return fmt.Errorf("unsupported format %q: use text, json, markdown, or github-pr", format)
			}

			b, err := loadSnapshotOrBuild(before)
			if err != nil {
				return fmt.Errorf("before (%q): %w", before, err)
			}
			a, snap, err := loadSnapshotOrBuildWithSnapshot(after)
			if err != nil {
				return fmt.Errorf("after (%q): %w", after, err)
			}

			r := comparison.CompareWithSnapshot(b, a, snap)

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() {
				_ = cleanup()
			}()

			driftThresholdBytes := report.MinorDriftThreshold
			if driftThreshold != "" {
				val, err := budget.ParseBytes(driftThreshold)
				if err != nil {
					return fmt.Errorf("invalid --drift-threshold: %w", err)
				}
				driftThresholdBytes = val
			}

			opts := report.TextOptions{
				Top:            top,
				Filter:         filter,
				All:            all,
				DriftThreshold: driftThresholdBytes,
				Project:        project,
			}

			if format == "json" {
				return report.JSON(w, r)
			} else if report.IsGitHubPRFormat(format) {
				return report.ComparisonGitHubPR(w, r, budget.CheckResult{Passed: true}, opts)
			} else if report.IsMarkdownFormat(format) {
				return report.ComparisonMarkdown(w, r, opts)
			}

			return report.ComparisonTextWithOptions(w, r, opts)
		},
	}

	c.Flags().StringVarP(&before, "before", "b", "", "Path to saved summary JSON before the change (defaults to active baseline if omitted)")
	c.Flags().StringVarP(&after, "after", "a", "", "Path to saved summary JSON or stats.json after the change")
	c.Flags().StringVarP(&project, "project", "p", "", "Project name to display in comparison reports")
	c.Flags().StringVarP(&format, "format", "f", "text", "Output format: text, json, or markdown")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to specified file path instead of stdout")
	c.Flags().IntVar(&top, "top", 10, "Number of top package changes to display in text mode")
	c.Flags().StringVar(&filter, "filter", "", "Filter package changes by name substring in text mode")
	c.Flags().BoolVar(&all, "all", false, "Display all package changes in text mode")
	c.Flags().StringVar(&driftThreshold, "drift-threshold", "1KB", "Byte threshold under which individual regression findings are collapsed as micro-drift (e.g. 1KB, 500B)")

	return c
}

func loadSnapshotOrBuild(pathOrName string) (*analysis.AnalysisResult, error) {
	res, _, err := loadSnapshotOrBuildWithSnapshot(pathOrName)
	return res, err
}

func loadSnapshotOrBuildWithSnapshot(pathOrName string) (*analysis.AnalysisResult, *snapshot.BundleSnapshot, error) {
	// 1. Try resolving via baseline manager
	resolved := baseline.ResolvePath(pathOrName)
	if fi, err := os.Stat(resolved); err == nil && !fi.IsDir() {
		res, err := comparison.Parse(resolved)
		if err == nil {
			return res, nil, nil
		}
	}

	// 2. If it is a directory, locate stats and dist in directory
	fi, err := os.Stat(pathOrName)
	if err != nil {
		return nil, nil, fmt.Errorf("read path %q: %w", pathOrName, err)
	}

	if fi.IsDir() {
		sFile, dDir, err := resolveBuildArtifactsInDir(pathOrName, "", "", "")
		if err != nil {
			return nil, nil, err
		}
		return runAnalysisWithOptions(sFile, dDir, true)
	}

	// 3. If it is a file (e.g. stats.json), parse as stats or summary
	res, err := comparison.Parse(pathOrName)
	if err == nil {
		return res, nil, nil
	}

	// Try running build analysis on raw stats.json
	distDir := filepath.Dir(pathOrName)
	browserDir := filepath.Join(distDir, "browser")
	if bfi, err := os.Stat(browserDir); err == nil && bfi.IsDir() {
		distDir = browserDir
	}
	return runAnalysisWithOptions(pathOrName, distDir, true)
}
