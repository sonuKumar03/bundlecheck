package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/sonuKumar03/bundlecheck/internal/advisor"
)

func SuggestText(w io.Writer, r *advisor.AdvisorResult, showGzip bool) error {
	var text strings.Builder
	table := tabwriter.NewWriter(&text, 0, 0, 2, ' ', 0)

	fmt.Fprintln(table, "Initial Bundle Culprits & Contributors")
	if len(r.Suggestions) == 0 {
		fmt.Fprintln(table, "No major culprits detected. Your initial bundle structure looks clean!")
		_ = table.Flush()
		_, err := io.WriteString(w, text.String())
		return err
	}

	savingsStr := formatBytes(r.TotalPotentialSavings)
	fmt.Fprintf(table, "Found %d contributor(s) with initial JS footprint of ~%s:\n", len(r.Suggestions), savingsStr)

	for i, s := range r.Suggestions {
		savingsDisplay := formatBytes(s.Savings)
		if showGzip && s.SavingsGzip > 0 {
			savingsDisplay += fmt.Sprintf(" (~%s gzip)", formatBytes(s.SavingsGzip))
		}

		fmt.Fprintf(table, "\n[%s] #%d: %s\n", s.Severity, i+1, s.Title)
		fmt.Fprintf(table, "  Initial JS Impact:\t~%s\n", savingsDisplay)
		if s.File != "" {
			fmt.Fprintf(table, "  Culprit / Importer:\t%s\n", s.File)
		}
		fmt.Fprintf(table, "  Reason:\t%s\n", s.Description)
		if s.Action != "" {
			fmt.Fprintf(table, "  Action:\t%s\n", s.Action)
		}
	}

	if err := table.Flush(); err != nil {
		return err
	}
	_, err := io.WriteString(w, text.String())
	return err
}
