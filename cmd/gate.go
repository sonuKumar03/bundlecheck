package cmd

import (
	"context"
	"fmt"

	"github.com/sonuKumar03/bundleradar/internal/budget"
	"github.com/sonuKumar03/bundleradar/internal/core/diff"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
	"github.com/spf13/cobra"
)

func newGateCommand() *cobra.Command {
	var (
		dist                string
		bundler             string
		format              string
		output              string
		against             string
		maxInitial          string
		maxTotal            string
		maxInitialDelta     string
		forbid              []string
		detectDuplicatePkgs bool
	)

	c := &cobra.Command{
		Use:   "gate [stats.json]",
		Short: "Validate bundle sizes, regressions, and architecture rules against policy budgets in CI",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			statsPath := ""
			if len(args) > 0 {
				statsPath = args[0]
			}
			if statsPath == "" {
				return fmt.Errorf("stats path is required")
			}

			client := bundleradar.New()
			ctx := context.Background()

			bundle, err := client.Scan(ctx, bundleradar.ScanOptions{
				StatsPath: statsPath,
				DistPath:  dist,
				Bundler:   bundler,
			})
			if err != nil {
				return fmt.Errorf("scan bundle: %w", err)
			}

			var diffResult *bundleradar.BundleDiff
			if against != "" {
				baseBundle, err := client.Scan(ctx, bundleradar.ScanOptions{
					StatsPath: against,
					Bundler:   bundler,
				})
				if err != nil {
					return fmt.Errorf("scan baseline bundle %q: %w", against, err)
				}
				diffResult = client.Diff(baseBundle, bundle, diff.Options{})
			}

			var pol bundleradar.Policy
			if maxInitial != "" {
				val, err := budget.ParseBytes(maxInitial)
				if err != nil {
					return fmt.Errorf("invalid --max-initial: %w", err)
				}
				pol.MaxInitial = &val
			}
			if maxTotal != "" {
				val, err := budget.ParseBytes(maxTotal)
				if err != nil {
					return fmt.Errorf("invalid --max-total: %w", err)
				}
				pol.MaxTotal = &val
			}
			if maxInitialDelta != "" {
				val, err := budget.ParseBytes(maxInitialDelta)
				if err != nil {
					return fmt.Errorf("invalid --max-initial-delta: %w", err)
				}
				pol.MaxInitialDelta = &val
			}
			pol.ForbiddenPkgs = forbid
			pol.DetectDuplicatePkgs = detectDuplicatePkgs

			evalRes := client.Gate(bundle, diffResult, pol)

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() { _ = cleanup() }()

			rep, err := client.Reporter(format)
			if err != nil {
				return err
			}

			if err := rep.Render(ctx, w, evalRes); err != nil {
				return err
			}

			if !evalRes.Passed {
				return &PolicyViolationError{Err: fmt.Errorf("bundle policy violation")}
			}

			return nil
		},
	}

	c.Flags().StringVarP(&dist, "dist", "d", "", "Path to emitted browser dist with index.html")
	c.Flags().StringVar(&bundler, "bundler", "", "Override bundler auto-detection")
	c.Flags().StringVarP(&format, "format", "f", "terminal", "Output format: terminal, markdown, github-pr, json")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to file path")
	c.Flags().StringVar(&against, "against", "", "Baseline stats/metafile to enforce delta limits")
	c.Flags().StringVar(&maxInitial, "max-initial", "", "Maximum allowed initial bundle size (e.g. 250KB, 1MB)")
	c.Flags().StringVar(&maxTotal, "max-total", "", "Maximum allowed total bundle size (e.g. 1.5MB)")
	c.Flags().StringVar(&maxInitialDelta, "max-initial-delta", "", "Maximum allowed increase vs baseline")
	c.Flags().StringSliceVar(&forbid, "forbid", nil, "Forbidden package names (e.g. moment,lodash)")
	c.Flags().BoolVar(&detectDuplicatePkgs, "detect-duplicate-pkgs", true, "Fail if multiple versions of the same package are bundled")

	return c
}
