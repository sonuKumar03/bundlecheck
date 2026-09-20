package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundlecheck/internal/advisor"
	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/budget"
	"github.com/sonuKumar03/bundlecheck/internal/build"
	"github.com/sonuKumar03/bundlecheck/internal/compression"
	"github.com/sonuKumar03/bundlecheck/internal/report"
)

func suggestCommand() *cobra.Command {
	var (
		stats      string
		dist       string
		project    string
		format     string
		output     string
		minSavings string
		severity   string
		showGzip   bool
	)

	c := &cobra.Command{
		Use:   "suggest [stats.json] [dist]",
		Short: "Analyze bundle and generate prioritized size optimization recommendations",
		Long: `Analyze the bundle graph and input import chains to discover actionable optimization opportunities.
Detects heavy third-party libraries suitable for dynamic imports, eager route components,
and duplicated package contributions.`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) > 0 && stats == "" {
				stats = args[0]
			}
			if len(args) > 1 && dist == "" {
				dist = args[1]
			}

			if format != "text" && format != "json" && !report.IsMarkdownFormat(format) {
				return fmt.Errorf("unsupported format %q: use text, json, or markdown", format)
			}

			var minSavingsBytes int64 = 1024
			if minSavings != "" {
				parsed, err := budget.ParseBytes(minSavings)
				if err != nil {
					return fmt.Errorf("invalid --min-savings: %w", err)
				}
				minSavingsBytes = parsed
			}

			sFile, dDir, err := resolveBuildArtifacts(stats, dist, project)
			if err != nil {
				return err
			}

			s, err := build.Load(sFile, dDir)
			if err != nil {
				return err
			}

			// Run analysis and conditional compression
			_, err = analysis.Analyze(s)
			if err != nil {
				return err
			}
			if format == "json" || showGzip {
				compression.AttachCompression(s, dDir)
			}

			advisorOpts := advisor.AdvisorOptions{
				MinSavings:     minSavingsBytes,
				SeverityFilter: severity,
			}
			advisorRes := advisor.Analyze(s, advisorOpts)

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() {
				_ = cleanup()
			}()

			if format == "json" {
				return report.JSON(w, advisorRes)
			} else if report.IsMarkdownFormat(format) {
				return report.SuggestMarkdown(w, advisorRes, showGzip)
			}

			return report.SuggestText(w, advisorRes, showGzip)
		},
	}

	c.Flags().StringVarP(&stats, "stats", "s", "", "Path to Angular/esbuild stats.json (auto-detected if omitted)")
	c.Flags().StringVarP(&dist, "dist", "d", "", "Path to emitted browser dist with index.html (auto-detected if omitted)")
	c.Flags().StringVarP(&project, "project", "p", "", "Project name for multi-project workspaces when auto-detecting")
	c.Flags().StringVarP(&format, "format", "f", "text", "Output format: text, json, or markdown")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to specified file path instead of stdout")
	c.Flags().StringVarP(&minSavings, "min-savings", "m", "1KB", "Minimum estimated initial savings threshold (e.g. 5KB, 1024B)")
	c.Flags().StringVar(&severity, "severity", "", "Filter suggestions by minimum severity level (HIGH, MEDIUM, LOW)")
	c.Flags().BoolVarP(&showGzip, "gzip", "g", false, "Include estimated Gzip wire transfer savings in report")

	return c
}
