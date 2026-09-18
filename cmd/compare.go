package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"bundlecheck/internal/baseline"
	"bundlecheck/internal/comparison"
	"bundlecheck/internal/report"
)

func compareCommand() *cobra.Command {
	var (
		before string
		after  string
		format string
		output string
		filter string
		top    int
		all    bool
	)

	c := &cobra.Command{
		Use:   "compare",
		Short: "Compare two saved summary JSON snapshots",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if before == "" {
				// Try default baseline if exists
				def := baseline.ResolvePath("")
				if fi, err := os.Stat(def); err == nil && !fi.IsDir() {
					before = def
				}
			}

			if before == "" || after == "" {
				return fmt.Errorf("--before and --after require nonempty paths")
			}
			if format != "text" && format != "json" {
				return fmt.Errorf("unsupported format %q: use text or json", format)
			}

			b, err := comparison.Parse(before)
			if err != nil {
				return fmt.Errorf("--before: %w", err)
			}
			a, err := comparison.Parse(after)
			if err != nil {
				return fmt.Errorf("--after: %w", err)
			}

			r := comparison.Compare(b, a)

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer cleanup()

			if format == "json" {
				return report.JSON(w, r)
			}

			opts := report.TextOptions{
				Top:    top,
				Filter: filter,
				All:    all,
			}
			return report.ComparisonTextWithOptions(w, r, opts)
		},
	}

	c.Flags().StringVarP(&before, "before", "b", "", "Path to saved summary JSON before the change (defaults to .bundlecheck/baseline.json if present)")
	c.Flags().StringVarP(&after, "after", "a", "", "Path to saved summary JSON after the change (required)")
	c.Flags().StringVarP(&format, "format", "f", "text", "Output format: text or json")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to specified file path instead of stdout")
	c.Flags().IntVar(&top, "top", 10, "Number of top package changes to display in text mode")
	c.Flags().StringVar(&filter, "filter", "", "Filter package changes by name substring in text mode")
	c.Flags().BoolVar(&all, "all", false, "Display all package changes in text mode")

	return c
}
