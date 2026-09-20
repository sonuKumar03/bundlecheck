package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundlecheck/internal/analysis"
)

// NewRootCommand creates and configures the root bundlecheck cobra.Command.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:     "bundlecheck",
		Short:   "Summarize, inspect, measure, compare, trace, advise, and check Angular browser JavaScript bundles",
		Version: analysis.ToolVersion,
		Long: `bundlecheck is a fast CLI and AI agent skill for Angular esbuild bundle analysis.
It calculates accurate initial vs. lazy JavaScript byte totals, attributes npm package sizes,
estimates Gzip wire transfer sizes, traces dependency import paths, generates optimization
recommendations, tracks baselines across changes, and enforces bundle size budgets in CI.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		summaryCommand(),
		workspaceCommand(),
		compareCommand(),
		baselineCommand(),
		measureCommand(),
		checkCommand(),
		inspectCommand(),
		whyCommand(),
		suggestCommand(),
		benchmarkCommand(),
		mcpCommand(),
	)
	return root
}

func Execute(args []string, stdout, stderr io.Writer) int {
	root := NewRootCommand()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(stderr, "bundlecheck: %v\n", err)
		return 1
	}
	return 0
}
