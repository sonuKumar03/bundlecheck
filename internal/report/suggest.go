package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"bundlecheck/internal/advisor"
)

func SuggestText(w io.Writer, r *advisor.AdvisorResult, showGzip bool) error {
	var text strings.Builder
	table := tabwriter.NewWriter(&text, 0, 0, 2, ' ', 0)

	fmt.Fprintln(table, "Bundle Optimization Recommendations")
	if len(r.Suggestions) == 0 {
		fmt.Fprintln(table, "No major optimization opportunities detected. Your bundle structure looks clean!")
		_ = table.Flush()
		_, err := io.WriteString(w, text.String())
		return err
	}

	savingsStr := formatBytes(r.TotalPotentialSavings)
	fmt.Fprintf(table, "Found %d optimization opportunity(ies) with potential initial JS savings of ~%s:\n", len(r.Suggestions), savingsStr)

	for i, s := range r.Suggestions {
		savingsDisplay := formatBytes(s.Savings)
		if showGzip && s.SavingsGzip > 0 {
			savingsDisplay += fmt.Sprintf(" (~%s gzip)", formatBytes(s.SavingsGzip))
		}

		fmt.Fprintf(table, "\n[%s] #%d: %s\n", s.Severity, i+1, s.Title)
		fmt.Fprintf(table, "  Potential Savings:\t~%s\n", savingsDisplay)
		if s.File != "" {
			fmt.Fprintf(table, "  Target / Importer:\t%s\n", s.File)
		}
		fmt.Fprintf(table, "  Rationale:\t%s\n", s.Description)
		fmt.Fprintf(table, "  Action:\t%s\n", s.Action)
	}

	if err := table.Flush(); err != nil {
		return err
	}
	_, err := io.WriteString(w, text.String())
	return err
}
