package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"bundlecheck/internal/budget"
)

func BudgetReport(w io.Writer, res budget.CheckResult) error {
	var text strings.Builder
	table := tabwriter.NewWriter(&text, 0, 0, 2, ' ', 0)
	if res.Passed {
		fmt.Fprintln(table, "Bundle Budget Check: PASSED")
	} else {
		fmt.Fprintln(table, "Bundle Budget Check: FAILED")
		fmt.Fprintln(table, "\nViolations:")
		fmt.Fprintln(table, "Metric\tActual\tLimit\tMessage")
		for _, v := range res.Violations {
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", v.Metric, formatBytes(v.Actual), formatBytes(v.Limit), v.Message)
		}
	}
	if err := table.Flush(); err != nil {
		return err
	}
	_, err := io.WriteString(w, text.String())
	return err
}
