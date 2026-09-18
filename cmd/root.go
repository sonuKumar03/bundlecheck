package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"bundlecheck/internal/analysis"
)

func Execute(args []string, stdout, stderr io.Writer) int {
	root := &cobra.Command{
		Use:     "bundlecheck",
		Short:   "Summarize, measure, compare, and check Angular browser JavaScript bundles",
		Version: analysis.ToolVersion,
		Long: `bundlecheck is a fast CLI and AI agent skill for Angular esbuild bundle analysis.
It calculates accurate initial vs. lazy JavaScript byte totals, attributes npm package sizes,
tracks baselines across changes, and enforces bundle size budgets in CI.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(
		summaryCommand(),
		compareCommand(),
		baselineCommand(),
		measureCommand(),
		checkCommand(),
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(stderr, "bundlecheck: %v\n", err)
		return 1
	}
	return 0
}
