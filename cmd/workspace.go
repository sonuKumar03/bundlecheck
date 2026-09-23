package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sonuKumar03/bundleradar/internal/adapters/workspaces"
	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
	"github.com/spf13/cobra"
)

func newWorkspaceCommand() *cobra.Command {
	var (
		root   string
		format string
		output string
		apps   []string
	)

	parent := &cobra.Command{
		Use:   "workspace",
		Short: "Discover and analyze applications across monorepos and multi-app workspaces",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all discovered application targets across the workspace",
		RunE: func(c *cobra.Command, args []string) error {
			if root == "" {
				root = "."
			}

			var targets []core.Target
			var err error

			if len(apps) > 0 {
				resolver := &workspaces.ExplicitResolver{Specs: apps}
				targets, err = resolver.Resolve(c.Context(), root)
			} else {
				targets, err = workspaces.DefaultRegistry().Resolve(c.Context(), root)
			}
			if err != nil {
				return err
			}

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() { _ = cleanup() }()

			fmt.Fprintf(w, "Discovered %d application target(s) in %q:\n", len(targets), root)
			for i, t := range targets {
				fmt.Fprintf(w, "  %d. %s (stats: %s)\n", i+1, t.Name, t.StatsPath)
			}
			return nil
		},
	}

	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan and summarize all discovered application targets in the workspace",
		RunE: func(c *cobra.Command, args []string) error {
			if root == "" {
				root = "."
			}

			var targets []core.Target
			var err error

			if len(apps) > 0 {
				resolver := &workspaces.ExplicitResolver{Specs: apps}
				targets, err = resolver.Resolve(c.Context(), root)
			} else {
				targets, err = workspaces.DefaultRegistry().Resolve(c.Context(), root)
			}
			if err != nil {
				return err
			}

			client := bundleradar.New()
			ctx := c.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() { _ = cleanup() }()

			if format == "json" {
				type TargetResult struct {
					Name         string `json:"name"`
					StatsPath    string `json:"statsPath"`
					InitialBytes int64  `json:"initialBytes"`
					AsyncBytes   int64  `json:"asyncBytes"`
					TotalBytes   int64  `json:"totalBytes"`
					ChunkCount   int    `json:"chunkCount"`
				}
				type WorkspaceResult struct {
					Targets           []TargetResult `json:"targets"`
					TotalInitialBytes int64          `json:"totalInitialBytes"`
					TotalAsyncBytes   int64          `json:"totalAsyncBytes"`
					TotalBytes        int64          `json:"totalBytes"`
				}

				res := WorkspaceResult{Targets: make([]TargetResult, 0, len(targets))}
				for _, t := range targets {
					b, err := client.Scan(ctx, bundleradar.ScanOptions{
						StatsPath: t.StatsPath,
						DistPath:  t.DistPath,
						Bundler:   t.Bundler,
					})
					if err != nil {
						continue
					}
					var chunkBytes int64
					for _, ch := range b.Chunks {
						chunkBytes += ch.SizeBytes
					}
					tr := TargetResult{
						Name:         t.Name,
						StatsPath:    t.StatsPath,
						InitialBytes: b.TotalInitialBytes(),
						AsyncBytes:   b.TotalAsyncBytes(),
						TotalBytes:   chunkBytes,
						ChunkCount:   len(b.Chunks),
					}
					res.Targets = append(res.Targets, tr)
					res.TotalInitialBytes += tr.InitialBytes
					res.TotalAsyncBytes += tr.AsyncBytes
					res.TotalBytes += tr.TotalBytes
				}
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}

			fmt.Fprintf(w, "\n⚡ WORKSPACE BUNDLE SCAN (%d targets)\n", len(targets))
			fmt.Fprintf(w, "-------------------------------------------------------------\n")
			for _, t := range targets {
				b, err := client.Scan(ctx, bundleradar.ScanOptions{
					StatsPath: t.StatsPath,
					DistPath:  t.DistPath,
					Bundler:   t.Bundler,
				})
				if err != nil {
					fmt.Fprintf(w, "❌ %-20s Error: %v\n", t.Name, err)
					continue
				}
				fmt.Fprintf(w, "✓ %-20s Initial: %d bytes | Async: %d bytes (%d chunks)\n",
					t.Name, b.TotalInitialBytes(), b.TotalAsyncBytes(), len(b.Chunks))
			}
			fmt.Fprintf(w, "\n")
			return nil
		},
	}

	parent.PersistentFlags().StringVar(&root, "root", ".", "Root directory of the workspace")
	parent.PersistentFlags().StringSliceVar(&apps, "app", nil, "Explicit application target mapping name=stats[:dist]")
	parent.PersistentFlags().StringVarP(&format, "format", "f", "terminal", "Output format: terminal, json")
	parent.PersistentFlags().StringVarP(&output, "output", "o", "", "Write output to file path")

	parent.AddCommand(listCmd, scanCmd)
	return parent
}
