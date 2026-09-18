package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"bundlecheck/internal/baseline"
	"bundlecheck/internal/report"
)

func baselineCommand() *cobra.Command {
	var (
		stats   string
		dist    string
		project string
		format  string
		output  string
	)

	c := &cobra.Command{
		Use:   "baseline",
		Short: "Capture and save baseline metrics from the current build",
		Long: `Capture and save baseline metrics from the current Angular build.
By default, the baseline summary is saved to .bundlecheck/baseline.json in the current directory.
Subsequent builds can be compared against this baseline using 'bundlecheck measure'.`,
		Args: cobra.NoArgs,
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

			targetFile := baseline.ResolvePath(output)
			if err := baseline.Save(targetFile, result); err != nil {
				return fmt.Errorf("save baseline: %w", err)
			}

			if format == "json" {
				return report.JSON(c.OutOrStdout(), result)
			}

			fmt.Fprintf(c.OutOrStdout(), "Successfully saved baseline metrics to %s\n\n", targetFile)
			return report.Text(c.OutOrStdout(), result)
		},
	}

	c.Flags().StringVarP(&stats, "stats", "s", "", "Path to Angular/esbuild stats.json (auto-detected if omitted)")
	c.Flags().StringVarP(&dist, "dist", "d", "", "Path to emitted browser dist with index.html (auto-detected if omitted)")
	c.Flags().StringVarP(&project, "project", "p", "", "Project name for multi-project workspaces when auto-detecting")
	c.Flags().StringVarP(&format, "format", "f", "text", "Output format: text or json")
	c.Flags().StringVarP(&output, "output", "o", baseline.DefaultBaselineFilename, "Path to save baseline JSON file")

	return c
}
