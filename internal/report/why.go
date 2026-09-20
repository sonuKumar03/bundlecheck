package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/sonuKumar03/bundlecheck/internal/graph"
)

func WhyText(w io.Writer, r *graph.WhyResult) error {
	var text strings.Builder
	table := tabwriter.NewWriter(&text, 0, 0, 2, ' ', 0)

	if !r.Found {
		fmt.Fprintf(table, "Target %q was not found in the bundle metadata.\n", r.Target)
		_ = table.Flush()
		_, err := io.WriteString(w, text.String())
		return err
	}

	name := r.Target
	if r.PackageName != "" {
		name = r.PackageName
	}

	fmt.Fprintf(table, "Dependency Trace for %q\n", name)
	fmt.Fprintf(table, "Initial JS:\t%s\nLazy JS:\t%s\nTotal JS:\t%s\n", formatBytes(r.InitialBytes), formatBytes(r.LazyBytes), formatBytes(r.TotalBytes))

	if len(r.Chains) == 0 {
		fmt.Fprintln(table, "\nNo direct import paths found from root entrypoints.")
	} else {
		fmt.Fprintf(table, "\nImport Chain(s) (%d found):\n", len(r.Chains))
		for i, chain := range r.Chains {
			status := "LAZY"
			if chain.Initial {
				status = "INITIAL"
			}
			fmt.Fprintf(table, "\n#%d (chunk: %s - %s, size: %s)\n", i+1, chain.Output, status, formatBytes(chain.BytesInChunk))
			for j, step := range chain.Path {
				indent := strings.Repeat("  ", j)
				prefix := ""
				if j > 0 {
					prefix = "└── "
				}
				fmt.Fprintf(table, "%s%s%s\n", indent, prefix, step)
			}
		}
	}

	if err := table.Flush(); err != nil {
		return err
	}
	_, err := io.WriteString(w, text.String())
	return err
}
