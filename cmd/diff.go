package cmd

import (
	"context"
	"fmt"

	"github.com/sonuKumar03/bundleradar/internal/budget"
	"github.com/sonuKumar03/bundleradar/internal/core/diff"
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
				return fmt.Errorf("--against <baseline_path> is required")
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

			baseBundle, err := client.Scan(ctx, bundleradar.ScanOptions{
				StatsPath: against,
				Bundler:   bundler,
			})
			if err != nil {
				return fmt.Errorf("scan baseline bundle %q: %w", against, err)
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

	c.Flags().StringVar(&against, "against", "", "Path to baseline stats/metafile JSON or git ref")
	c.Flags().StringVarP(&format, "format", "f", "terminal", "Output format: terminal, markdown, github-pr, json")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to file path")
	c.Flags().StringVar(&driftThreshold, "drift-threshold", "1KB", "Byte threshold to bucket micro-drift")
	c.Flags().StringVar(&bundler, "bundler", "", "Override bundler auto-detection")

	return c
}
