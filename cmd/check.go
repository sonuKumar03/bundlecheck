package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"bundlecheck/internal/baseline"
	"bundlecheck/internal/budget"
	"bundlecheck/internal/comparison"
	"bundlecheck/internal/report"
)

func checkCommand() *cobra.Command {
	var (
		stats           string
		dist            string
		project         string
		baselinePath    string
		format          string
		maxInitial      string
		maxLazy         string
		maxTotal        string
		maxInitialDelta string
		maxTotalDelta   string
	)

	c := &cobra.Command{
		Use:   "check",
		Short: "Validate bundle sizes or regressions against budget thresholds",
		Long: `Validate bundle sizes or regressions against specified budget thresholds for CI and local verification.
Returns exit code 0 if all budgets pass, or exit code 1 if any threshold is breached.`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if format != "text" && format != "json" {
				return fmt.Errorf("unsupported format %q: use text or json", format)
			}

			// Parse limits
			var limits budget.Limits
			if maxInitial != "" {
				val, err := budget.ParseBytes(maxInitial)
				if err != nil {
					return fmt.Errorf("invalid --max-initial: %w", err)
				}
				limits.MaxInitial = &val
			}
			if maxLazy != "" {
				val, err := budget.ParseBytes(maxLazy)
				if err != nil {
					return fmt.Errorf("invalid --max-lazy: %w", err)
				}
				limits.MaxLazy = &val
			}
			if maxTotal != "" {
				val, err := budget.ParseBytes(maxTotal)
				if err != nil {
					return fmt.Errorf("invalid --max-total: %w", err)
				}
				limits.MaxTotal = &val
			}
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

			if limits.MaxInitial == nil && limits.MaxLazy == nil && limits.MaxTotal == nil &&
				limits.MaxInitialDelta == nil && limits.MaxTotalDelta == nil {
				return fmt.Errorf("at least one budget threshold must be specified (e.g. --max-initial 200KB)")
			}

			// Analyze current build
			sFile, dDir, err := resolveBuildArtifacts(stats, dist, project)
			if err != nil {
				return err
			}
			currentResult, err := runAnalysis(sFile, dDir)
			if err != nil {
				return err
			}

			var checkResult budget.CheckResult

			// If delta limits are specified or baseline is passed, perform comparison check
			if limits.MaxInitialDelta != nil || limits.MaxTotalDelta != nil || baselinePath != "" {
				baseResult, err := baseline.Load(baselinePath)
				if err != nil {
					return err
				}
				compResult := comparison.Compare(baseResult, currentResult)
				checkResult = budget.CheckComparison(compResult, limits)
			} else {
				checkResult = budget.CheckSummary(currentResult.Summary, limits)
			}

			if format == "json" {
				if err := report.JSON(c.OutOrStdout(), checkResult); err != nil {
					return err
				}
			} else {
				if err := report.BudgetReport(c.OutOrStdout(), checkResult); err != nil {
					return err
				}
			}

			if !checkResult.Passed {
				return fmt.Errorf("bundle budget check failed (%d violations)", len(checkResult.Violations))
			}

			return nil
		},
	}

	c.Flags().StringVarP(&stats, "stats", "s", "", "Path to Angular/esbuild stats.json (auto-detected if omitted)")
	c.Flags().StringVarP(&dist, "dist", "d", "", "Path to emitted browser dist with index.html (auto-detected if omitted)")
	c.Flags().StringVarP(&project, "project", "p", "", "Project name for multi-project workspaces when auto-detecting")
	c.Flags().StringVarP(&baselinePath, "baseline", "b", "", "Path to baseline summary JSON for regression checks")
	c.Flags().StringVarP(&format, "format", "f", "text", "Output format: text or json")
	c.Flags().StringVar(&maxInitial, "max-initial", "", "Maximum allowed initial JS size (e.g. 250KB, 1MB)")
	c.Flags().StringVar(&maxLazy, "max-lazy", "", "Maximum allowed lazy JS size (e.g. 500KB)")
	c.Flags().StringVar(&maxTotal, "max-total", "", "Maximum allowed total JS size (e.g. 1.5MB)")
	c.Flags().StringVar(&maxInitialDelta, "max-initial-delta", "", "Maximum allowed increase in initial JS vs baseline (e.g. 0B, 10KB)")
	c.Flags().StringVar(&maxTotalDelta, "max-total-delta", "", "Maximum allowed increase in total JS vs baseline (e.g. 50KB)")

	return c
}
