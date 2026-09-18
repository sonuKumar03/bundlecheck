package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"bundlecheck/internal/angular"
	"bundlecheck/internal/artifact"
	"bundlecheck/internal/graph"
	"bundlecheck/internal/report"
)

func whyCommand() *cobra.Command {
	var (
		stats       string
		dist        string
		project     string
		pkgName     string
		format      string
		output      string
		maxChains   int
		initialOnly bool
	)

	c := &cobra.Command{
		Use:   "why [stats.json] <package>",
		Short: "Trace why a package or module is included in the bundle",
		Long: `Trace the static import chains from bootstrap root entrypoints to a target package or module.
Explains whether the package is pulled into initial or lazy JavaScript and shows exact source file import paths.`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			target := pkgName
			if len(args) == 1 {
				target = args[0]
			} else if len(args) == 2 {
				if stats == "" {
					stats = args[0]
				}
				target = args[1]
			}
			if target == "" {
				return fmt.Errorf("target package or file path required (e.g. 'bundlecheck why lodash' or 'bundlecheck why dist/stats.json lodash')")
			}
			if format != "text" && format != "json" {
				return fmt.Errorf("unsupported format %q: use text or json", format)
			}

			sFile, dDir, err := resolveBuildArtifacts(stats, dist, project)
			if err != nil {
				return err
			}

			meta, err := angular.Parse(sFile)
			if err != nil {
				return err
			}
			s, err := angular.Normalize(meta)
			if err != nil {
				return err
			}
			outputs, roots, err := artifact.BrowserOutputs(s.Outputs, dDir)
			if err != nil {
				return err
			}
			if err := graph.Classify(outputs, roots); err != nil {
				return err
			}
			s.Outputs = outputs

			whyResult, err := graph.TracePackage(s, target, initialOnly, maxChains)
			if err != nil {
				return err
			}

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() {
				_ = cleanup()
			}()

			if format == "json" {
				return report.JSON(w, whyResult)
			}

			return report.WhyText(w, whyResult)
		},
	}

	c.Flags().StringVarP(&stats, "stats", "s", "", "Path to Angular/esbuild stats.json (auto-detected if omitted)")
	c.Flags().StringVarP(&dist, "dist", "d", "", "Path to emitted browser dist with index.html (auto-detected if omitted)")
	c.Flags().StringVarP(&project, "project", "p", "", "Project name for multi-project workspaces when auto-detecting")
	c.Flags().StringVar(&pkgName, "package", "", "Target package or module name to trace (alternative to positional argument)")
	c.Flags().StringVarP(&format, "format", "f", "text", "Output format: text or json")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to specified file path instead of stdout")
	c.Flags().IntVar(&maxChains, "max-chains", 5, "Maximum number of distinct import chains to display")
	c.Flags().BoolVar(&initialOnly, "initial-only", false, "Show only import chains leading to initial JS outputs")

	return c
}
