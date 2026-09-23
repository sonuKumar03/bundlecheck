package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/sonuKumar03/bundleradar/internal/comparison"
)

func ComparisonText(w io.Writer, r *comparison.Result) error {
	return ComparisonTextWithOptions(w, r, TextOptions{Top: 10})
}

func ComparisonTextWithOptions(w io.Writer, r *comparison.Result, opts TextOptions) error {
	var text strings.Builder
	table := tabwriter.NewWriter(&text, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "Angular Bundle Comparison")
	fmt.Fprintln(table, "\tBefore\tAfter\tChange")
	fmt.Fprintf(table, "Initial JS\t%s\t%s\t%s\n", formatBytes(r.Summary.Before.InitialJS), formatBytes(r.Summary.After.InitialJS), formatDelta(r.Summary.Delta.InitialJS))
	fmt.Fprintf(table, "Lazy JS\t%s\t%s\t%s\n", formatBytes(r.Summary.Before.LazyJS), formatBytes(r.Summary.After.LazyJS), formatDelta(r.Summary.Delta.LazyJS))
	fmt.Fprintf(table, "Total JS\t%s\t%s\t%s\n", formatBytes(r.Summary.Before.TotalJS), formatBytes(r.Summary.After.TotalJS), formatDelta(r.Summary.Delta.TotalJS))

	header := "\nLargest package changes"
	if opts.Filter != "" {
		header = fmt.Sprintf("\nPackage changes matching %q", opts.Filter)
	}
	fmt.Fprintln(table, header)
	fmt.Fprintln(table, "Package\tStatus\tInitial change\tLazy change\tTotal change")

	shown := 0
	limit := opts.Top
	if limit <= 0 {
		limit = 10
	}
	if opts.All {
		limit = len(r.Packages)
	}

	for _, p := range r.Packages {
		if p.Status == "unchanged" && opts.Filter == "" {
			continue
		}
		if opts.Filter != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(opts.Filter)) {
			continue
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", p.Name, p.Status, formatDelta(p.Delta.InitialBytes), formatDelta(p.Delta.LazyBytes), formatDelta(p.Delta.TotalBytes))
		shown++
		if shown >= limit {
			break
		}
	}
	if shown == 0 {
		fmt.Fprintln(table, "(none)")
	}
	if len(r.Findings) > 0 {
		fmt.Fprintln(table, "\nRegression explanation")
		for _, f := range r.Findings {
			target := f.Name
			if f.Kind == "source" {
				target = "[source] " + target
			}
			if len(f.Chunks) > 0 {
				target = fmt.Sprintf("%s -> %s", target, strings.Join(f.Chunks, ", "))
			}
			if f.Reason != "" {
				target = fmt.Sprintf("%s (%s)", target, f.Reason)
			}
			fmt.Fprintf(table, "%s\t%s\n", formatDelta(f.DeltaBytes), target)
			for i, step := range f.TracePath {
				prefix := "  "
				if i > 0 {
					prefix = "  -> "
				}
				fmt.Fprintf(table, "\t%s%s\n", prefix, step)
			}
		}
	}
	if err := table.Flush(); err != nil {
		return err
	}
	_, err := io.WriteString(w, text.String())
	return err
}

func formatDelta(n int64) string {
	if n < 0 {
		return "-" + formatBytes(-n)
	}
	if n > 0 {
		return "+" + formatBytes(n)
	}
	return formatBytes(0)
}
