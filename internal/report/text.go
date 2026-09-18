// Package report renders structured results without deriving bundle facts.
package report

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"bundlecheck/internal/analysis"
)

type TextOptions struct {
	Top    int
	Filter string
	All    bool
}

func Text(w io.Writer, r *analysis.AnalysisResult) error {
	return TextWithOptions(w, r, TextOptions{Top: 10})
}

func TextWithOptions(w io.Writer, r *analysis.AnalysisResult, opts TextOptions) error {
	var text strings.Builder
	table := tabwriter.NewWriter(&text, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "Angular Bundle Summary")
	fmt.Fprintf(table, "Initial JS\t%s\nLazy JS\t%s\nTotal JS\t%s\n", formatBytes(r.Summary.InitialJS), formatBytes(r.Summary.LazyJS), formatBytes(r.Summary.TotalJS))

	header := "\nLargest initial packages"
	if opts.Filter != "" {
		header = fmt.Sprintf("\nInitial packages matching %q", opts.Filter)
	}
	fmt.Fprintln(table, header)

	shown := 0
	limit := opts.Top
	if limit <= 0 {
		limit = 10
	}
	if opts.All {
		limit = len(r.Packages)
	}

	for _, p := range r.Packages {
		if p.InitialBytes == 0 {
			continue
		}
		if opts.Filter != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(opts.Filter)) {
			continue
		}

		pct := ""
		if r.Summary.InitialJS > 0 {
			share := (float64(p.InitialBytes) / float64(r.Summary.InitialJS)) * 100
			pct = fmt.Sprintf("(%.1f%%)", share)
		}

		if pct != "" {
			fmt.Fprintf(table, "%s\t%s\t%s\n", p.Name, formatBytes(p.InitialBytes), pct)
		} else {
			fmt.Fprintf(table, "%s\t%s\n", p.Name, formatBytes(p.InitialBytes))
		}
		shown++
		if shown >= limit {
			break
		}
	}
	if shown == 0 {
		fmt.Fprintln(table, "(none)")
	}
	if err := table.Flush(); err != nil {
		return err
	}
	_, err := io.WriteString(w, text.String())
	return err
}

func formatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	value, unit := float64(n), 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	number := strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 2, 64), "0"), ".")
	return number + " " + units[unit]
}
