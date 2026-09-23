package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundleradar/internal/baseline"
	"github.com/sonuKumar03/bundleradar/internal/budget"
	"github.com/sonuKumar03/bundleradar/internal/comparison"
	"github.com/sonuKumar03/bundleradar/internal/report"
)

func measureCommand() *cobra.Command {
	var (
		stats           string
		dist            string
		project         string
		entry           string
		baselinePath    string
		format          string
		output          string
		filter          string
		top             int
		all             bool
		maxInitialDelta string
		maxTotalDelta   string
		driftThreshold  string
	)

	c := &cobra.Command{
		Use:   "measure",
		Short: "Measure current build changes against a saved baseline",
		Long: `Measure current build changes against a baseline summary (defaults to .bundleradar/baseline.json).
Reports initial/lazy/total JS deltas and package movements.
Optionally verifies that size regressions do not exceed specified limits.`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if format != "text" && format != "json" && !report.IsMarkdownFormat(format) && !report.IsGitHubPRFormat(format) {
				return fmt.Errorf("unsupported format %q: use text, json, markdown, or github-pr", format)
			}

			// 1. Load baseline
			baseResult, err := baseline.Load(baselinePath)
			if err != nil {
				return err
			}

			// 2. Analyze current build
			sFile, dDir, err := resolveBuildArtifacts(stats, dist, project)
			if err != nil {
				return err
			}
			currentResult, currentSnap, err := runAnalysisWithEntry(sFile, dDir, entry, true)
			if err != nil {
				return err
			}

			// 3. Compare baseline vs current
			compResult := comparison.CompareWithSnapshot(baseResult, currentResult, currentSnap)

			// 4. Enforce regression delta budgets
			var limits budget.Limits
			if maxInitialDelta != "" {
				val, err := budget.ParseBytes(maxInitialDelta)
				if err != nil {
					return fmt.Errorf("invalid --max-initial-delta: %w", err)
				}
				limits.MaxInitialDelta = &val
			}
			if maxTotalDelta != "" {
				val, err := budget.ParseBytes(maxTotalDelta)
				if err != nil {
					return fmt.Errorf("invalid --max-total-delta: %w", err)
				}
				limits.MaxTotalDelta = &val
			}

			budgetCheck := budget.CheckComparison(compResult, limits)

			// 5. Output results
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
				if err := report.JSON(w, compResult); err != nil {
					return err
				}
			} else if report.IsGitHubPRFormat(format) {
				if err := report.ComparisonGitHubPR(w, compResult, budgetCheck, opts); err != nil {
					return err
				}
			} else if report.IsMarkdownFormat(format) {
				if err := report.ComparisonMarkdown(w, compResult, opts); err != nil {
					return err
				}
			} else {
				if err := report.ComparisonTextWithOptions(w, compResult, opts); err != nil {
					return err
				}
				if len(budgetCheck.Violations) > 0 {
					fmt.Fprintln(c.ErrOrStderr())
					_ = report.BudgetReport(c.ErrOrStderr(), budgetCheck)
				}
			}

			if !budgetCheck.Passed {
				return &PolicyViolationError{Err: fmt.Errorf("bundle regression limits breached (%d violations)", len(budgetCheck.Violations))}
			}

			return nil
		},
	}

	c.Flags().StringVarP(&stats, "stats", "s", "", "Path to Angular/esbuild stats.json (auto-detected if omitted)")
	c.Flags().StringVarP(&dist, "dist", "d", "", "Path to emitted browser dist with index.html (auto-detected if omitted)")
	c.Flags().StringVarP(&project, "project", "p", "", "Project name for multi-project workspaces when auto-detecting")
	c.Flags().StringVarP(&entry, "entry", "e", "", "Scope analysis to a specific entrypoint file or chunk name")
	c.Flags().StringVarP(&baselinePath, "baseline", "b", baseline.DefaultBaselineFilename, "Path to baseline summary JSON")
	c.Flags().StringVarP(&format, "format", "f", "text", "Output format: text, json, markdown, or github-pr")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to specified file path instead of stdout")
	c.Flags().IntVar(&top, "top", 10, "Number of top package changes to display in text mode")
	c.Flags().StringVar(&filter, "filter", "", "Filter package changes by name substring in text mode")
	c.Flags().BoolVar(&all, "all", false, "Display all package changes in text mode")
	c.Flags().StringVar(&maxInitialDelta, "max-initial-delta", "", "Maximum allowed increase in initial JS (e.g. 0B, 10KB)")
	c.Flags().StringVar(&maxTotalDelta, "max-total-delta", "", "Maximum allowed increase in total JS (e.g. 50KB)")
	c.Flags().StringVar(&driftThreshold, "drift-threshold", "1KB", "Byte threshold under which individual regression findings are collapsed as micro-drift (e.g. 1KB, 500B)")

	return c
}
