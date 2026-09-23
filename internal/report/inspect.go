package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/sonuKumar03/bundleradar/internal/analysis"
)

func InspectText(w io.Writer, r *analysis.InspectResult) error {
	var text strings.Builder
	table := tabwriter.NewWriter(&text, 0, 0, 2, ' ', 0)
	status := "LAZY"
	if r.Initial {
		status = "INITIAL"
	}
	fmt.Fprintln(table, "Chunk Inspection")
	fmt.Fprintf(table, "Chunk\t%s\nLoading\t%s\nSize\t%s\n", r.Chunk, status, formatBytes(r.Bytes))
	if r.EntryPoint != "" {
		fmt.Fprintf(table, "Entry point\t%s\n", r.EntryPoint)
	}
	writeContributors(table, "Packages", r.Packages)
	writeContributors(table, "Modules", r.Modules)
	if err := table.Flush(); err != nil {
		return err
	}
	_, err := io.WriteString(w, text.String())
	return err
}

func writeContributors(w io.Writer, heading string, items []analysis.Contributor) {
	fmt.Fprintf(w, "\n%s\n", heading)
	if len(items) == 0 {
		fmt.Fprintln(w, "(none)")
		return
	}
	for _, item := range items {
		fmt.Fprintf(w, "%s\t%s\n", item.Name, formatBytes(item.Bytes))
	}
}
