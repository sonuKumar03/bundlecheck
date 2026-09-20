package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/build"
	"bundlecheck/internal/report"
)

func inspectCommand() *cobra.Command {
	var stats, dist, project, format, output string
	c := &cobra.Command{
		Use:   "inspect <chunk>",
		Short: "Inspect the contributors to one emitted JavaScript chunk",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if format != "text" && format != "json" {
				return fmt.Errorf("unsupported format %q: use text or json", format)
			}
			sFile, dDir, err := resolveBuildArtifacts(stats, dist, project)
			if err != nil {
				return err
			}
			s, err := build.Load(sFile, dDir)
			if err != nil {
				return err
			}
			result, err := analysis.InspectChunk(s, args[0])
			if err != nil {
				return err
			}
			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() { _ = cleanup() }()
			if format == "json" {
				return report.JSON(w, result)
			}
			return report.InspectText(w, result)
		},
	}
	c.Flags().StringVarP(&stats, "stats", "s", "", "Path to Angular/esbuild stats.json (auto-detected if omitted)")
	c.Flags().StringVarP(&dist, "dist", "d", "", "Path to emitted browser dist with index.html (auto-detected if omitted)")
	c.Flags().StringVarP(&project, "project", "p", "", "Project name for multi-project workspaces when auto-detecting")
	c.Flags().StringVarP(&format, "format", "f", "text", "Output format: text or json")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to specified file path instead of stdout")
	return c
}
