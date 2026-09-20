package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"bundlecheck/internal/report"
)

func benchmarkCommand() *cobra.Command {
	var format, output string

	c := &cobra.Command{
		Use:   "benchmark [benchmark-output.txt]",
		Short: "Format and display raw Go benchmark metrics in human-readable units (µs/ms, KB/MB)",
		Long: `Parse and format the output of 'go test -bench=. -benchmem' into human-readable units.
Converts raw nanoseconds (ns/op) to microseconds (µs) and milliseconds (ms), and bytes (B/op) to KB / MB.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if format != "text" && format != "json" && !report.IsMarkdownFormat(format) {
				return fmt.Errorf("unsupported format %q: use text, json, or markdown", format)
			}

			var file *os.File
			var err error

			if len(args) > 0 {
				file, err = os.Open(args[0])
				if err != nil {
					return fmt.Errorf("open benchmark file %q: %w", args[0], err)
				}
				defer file.Close()
			} else {
				// Read from stdin if available or look for benchmark-output.txt in current directory
				stat, _ := os.Stdin.Stat()
				if (stat.Mode() & os.ModeCharDevice) == 0 {
					file = os.Stdin
				} else {
					defaultFile := "benchmark-output.txt"
					file, err = os.Open(defaultFile)
					if err != nil {
						return fmt.Errorf("no benchmark input provided; run 'go test -bench=. -benchmem ./... > benchmark-output.txt' first or pipe via stdin")
					}
					defer file.Close()
				}
			}

			metrics := report.ParseBenchmarkOutput(file)
			if len(metrics) == 0 {
				return fmt.Errorf("no benchmark results found in input")
			}

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() { _ = cleanup() }()

			if format == "json" {
				return report.JSON(w, metrics)
			} else if report.IsMarkdownFormat(format) {
				return report.BenchmarkMarkdown(w, metrics)
			}
			return report.BenchmarkText(w, metrics)
		},
	}

	c.Flags().StringVarP(&format, "format", "f", "text", "Output format: text, json, or markdown")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to specified file path instead of stdout")

	return c
}
