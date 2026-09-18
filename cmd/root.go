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
		Short:   "Summarize, measure, compare, trace, advise, and check Angular browser JavaScript bundles",
		Version: analysis.ToolVersion,
		Long: `bundlecheck is a fast CLI and AI agent skill for Angular esbuild bundle analysis.
It calculates accurate initial vs. lazy JavaScript byte totals, attributes npm package sizes,
estimates Gzip wire transfer sizes, traces dependency import paths, generates optimization
recommendations, tracks baselines across changes, and enforces bundle size budgets in CI.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(
		summaryCommand(),
		workspaceCommand(),
		compareCommand(),
		baselineCommand(),
		measureCommand(),
		checkCommand(),
		whyCommand(),
		suggestCommand(),
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(stderr, "bundlecheck: %v\n", err)
		return 1
	}
	return 0
}
