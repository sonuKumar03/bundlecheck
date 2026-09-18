package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/angular"
	"bundlecheck/internal/artifact"
	"bundlecheck/internal/discovery"
	"bundlecheck/internal/graph"
	"bundlecheck/internal/report"
)

func summaryCommand() *cobra.Command {
	var (
		stats   string
		dist    string
		project string
		format  string
		output  string
		filter  string
		top     int
		all     bool
	)

	c := &cobra.Command{
		Use:   "summary",
		Short: "Report initial and lazy JS sizes and npm contributors",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if format != "text" && format != "json" {
				return fmt.Errorf("unsupported format %q: use text or json", format)
			}

			sFile, dDir, err := resolveBuildArtifacts(stats, dist, project)
			if err != nil {
				return err
			}

			result, err := runAnalysis(sFile, dDir)
			if err != nil {
				return err
			}

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer cleanup()

			if format == "json" {
				return report.JSON(w, result)
			}

			opts := report.TextOptions{
				Top:    top,
				Filter: filter,
				All:    all,
			}
			return report.TextWithOptions(w, result, opts)
		},
	}

	c.Flags().StringVarP(&stats, "stats", "s", "", "Path to Angular/esbuild stats.json (auto-detected if omitted)")
	c.Flags().StringVarP(&dist, "dist", "d", "", "Path to emitted browser dist with index.html (auto-detected if omitted)")
	c.Flags().StringVarP(&project, "project", "p", "", "Project name for multi-project workspaces when auto-detecting")
	c.Flags().StringVarP(&format, "format", "f", "text", "Output format: text or json")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to specified file path instead of stdout")
	c.Flags().IntVar(&top, "top", 10, "Number of top packages to display in text mode")
	c.Flags().StringVar(&filter, "filter", "", "Filter packages by name substring in text mode")
	c.Flags().BoolVar(&all, "all", false, "Display all packages in text mode")

	return c
}

func resolveBuildArtifacts(stats, dist, project string) (string, string, error) {
	if stats != "" && dist != "" {
		return stats, dist, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("get working directory: %w", err)
	}

	discoveredStats, discoveredDist, err := discovery.Locate(wd, project)
	if err != nil {
		if stats == "" && dist == "" {
			return "", "", err
		}
		if stats == "" {
			return "", "", fmt.Errorf("missing --stats file path: %w", err)
		}
		if dist == "" {
			return "", "", fmt.Errorf("missing --dist directory path: %w", err)
		}
	}

	if stats == "" {
		stats = discoveredStats
	}
	if dist == "" {
		dist = discoveredDist
	}

	if stats == "" || dist == "" {
		return "", "", fmt.Errorf("--stats and --dist require nonempty paths")
	}

	return stats, dist, nil
}

func runAnalysis(stats, dist string) (*analysis.AnalysisResult, error) {
	meta, err := angular.Parse(stats)
	if err != nil {
		return nil, err
	}
	s, err := angular.Normalize(meta)
	if err != nil {
		return nil, err
	}
	outputs, roots, err := artifact.BrowserOutputs(s.Outputs, dist)
	if err != nil {
		return nil, err
	}
	if err := graph.Classify(outputs, roots); err != nil {
		return nil, err
	}
	s.Outputs = outputs
	return analysis.Analyze(s)
}

func getOutputWriter(c *cobra.Command, output string) (io.Writer, func() error, error) {
	if output == "" {
		return c.OutOrStdout(), func() error { return nil }, nil
	}
	dir := filepath.Dir(output)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, nil, fmt.Errorf("create directory %q: %w", dir, err)
		}
	}
	f, err := os.Create(output)
	if err != nil {
		return nil, nil, fmt.Errorf("create output file %q: %w", output, err)
	}
	return f, f.Close, nil
}
