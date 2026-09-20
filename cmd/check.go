package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundlecheck/internal/baseline"
	"github.com/sonuKumar03/bundlecheck/internal/budget"
	"github.com/sonuKumar03/bundlecheck/internal/comparison"
	"github.com/sonuKumar03/bundlecheck/internal/config"
	"github.com/sonuKumar03/bundlecheck/internal/report"
)

func checkCommand() *cobra.Command {
	var (
		stats           string
		dist            string
		project         string
		baselinePath    string
		configFile      string
		format          string
		output          string
		maxInitial      string
		maxLazy         string
		maxTotal        string
		maxInitialDelta string
		maxTotalDelta   string
	)

	c := &cobra.Command{
		Use:   "check [stats.json] [dist]",
		Short: "Validate bundle sizes, regressions, and repository rules against budget thresholds",
		Long: `Validate bundle sizes or regressions against specified budget thresholds for CI and local verification.
Automatically loads project budgets and package rules from .bundlecheck.yml if present.
Returns exit code 0 if all budgets and rules pass, or exit code 1 if any threshold is breached.`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) > 0 && stats == "" {
				stats = args[0]
			}
			if len(args) > 1 && dist == "" {
				dist = args[1]
			}

			if format != "text" && format != "json" && !report.IsMarkdownFormat(format) && !report.IsGitHubPRFormat(format) {
				return fmt.Errorf("unsupported format %q: use text, json, markdown, or github-pr", format)
			}

			// 1. Load config file if present or specified
			var cfg *config.Config
			if configFile != "" {
				var err error
				cfg, err = config.LoadFile(configFile)
				if err != nil {
					return fmt.Errorf("load config %q: %w", configFile, err)
				}
			} else {
				var err error
				cfg, configFile, err = config.FindAndLoad("")
				if err != nil {
					return fmt.Errorf("load config %q: %w", configFile, err)
				}
			}

			// 2. Parse CLI limits
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

			// 3. Apply config file defaults to limits
			if cfg != nil {
				if err := cfg.ApplyToLimits(&limits); err != nil {
					return err
				}
			}

			// 4. Analyze current build
			sFile, dDir, err := resolveBuildArtifacts(stats, dist, project)
			if err != nil {
				return err
			}
			currentResult, _, err := runAnalysisWithOptions(sFile, dDir, false)
			if err != nil {
				return err
			}

			var checkResult budget.CheckResult

			// 5. Run budget check (absolute or delta)
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

			// 6. Check repository package rules from config
			if cfg != nil {
				ruleViolations := cfg.CheckRules(currentResult)
				if len(ruleViolations) > 0 {
					checkResult.Passed = false
					checkResult.Violations = append(checkResult.Violations, ruleViolations...)
				}
			}

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() {
				_ = cleanup()
			}()

			if format == "json" {
				if err := report.JSON(w, checkResult); err != nil {
					return err
				}
			} else if report.IsGitHubPRFormat(format) {
				if err := report.CheckGitHubPR(w, checkResult); err != nil {
					return err
				}
			} else if report.IsMarkdownFormat(format) {
				if err := report.CheckMarkdown(w, checkResult); err != nil {
					return err
				}
			} else {
				if err := report.BudgetReport(w, checkResult); err != nil {
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
	c.Flags().StringVarP(&configFile, "config", "c", "", "Path to .bundlecheck.yml configuration file")
	c.Flags().StringVarP(&format, "format", "f", "text", "Output format: text, json, markdown, or github-pr")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to file instead of stdout")
	c.Flags().StringVar(&maxInitial, "max-initial", "", "Maximum allowed initial JS size (e.g. 250KB, 1MB)")
	c.Flags().StringVar(&maxLazy, "max-lazy", "", "Maximum allowed lazy JS size (e.g. 500KB)")
	c.Flags().StringVar(&maxTotal, "max-total", "", "Maximum allowed total JS size (e.g. 1.5MB)")
	c.Flags().StringVar(&maxInitialDelta, "max-initial-delta", "", "Maximum allowed increase in initial JS vs baseline (e.g. 0B, 10KB)")
	c.Flags().StringVar(&maxTotalDelta, "max-total-delta", "", "Maximum allowed increase in total JS vs baseline (e.g. 50KB)")

	return c
}
