package cmd

import (
	"context"
	"fmt"

	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
	"github.com/spf13/cobra"
)

func newScanCommand() *cobra.Command {
	var (
		dist    string
		bundler string
		format  string
		output  string
		entry   string
		why     string
		top     int
	)

	c := &cobra.Command{
		Use:   "scan [stats.json]",
		Short: "Inspect bundle sizes, breakdown, and package dependencies across entrypoints",
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
				return err
			}

			if entry != "" {
				if _, ok := bundle.Entrypoints[entry]; !ok {
					return fmt.Errorf("entrypoint %q not found in bundle", entry)
				}
			}

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() { _ = cleanup() }()

			rep, err := client.Reporter(format)
			if err != nil {
				return err
			}

			return rep.Render(ctx, w, bundle)
		},
	}

	c.Flags().StringVarP(&dist, "dist", "d", "", "Path to emitted browser dist with index.html")
	c.Flags().StringVar(&bundler, "bundler", "", "Override bundler auto-detection (esbuild, angular, vite, webpack)")
	c.Flags().StringVarP(&format, "format", "f", "terminal", "Output format: terminal, markdown, json")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to file path")
	c.Flags().StringVarP(&entry, "entry", "e", "", "Scope scan to a specific entrypoint")
	c.Flags().StringVar(&why, "why", "", "Trace import path root for a specific package")
	c.Flags().IntVar(&top, "top", 10, "Number of top packages to list")

	return c
}
