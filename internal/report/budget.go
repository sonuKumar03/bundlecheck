package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/sonuKumar03/bundleradar/internal/budget"
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

// MultiAppBudgetReport renders text output for multi-app budget checks.
func MultiAppBudgetReport(w io.Writer, res budget.MultiAppCheckResult) error {
	var text strings.Builder
	table := tabwriter.NewWriter(&text, 0, 0, 2, ' ', 0)
	status := "PASSED"
	if !res.Passed {
		status = "FAILED"
	}
	fmt.Fprintf(table, "BundleRadar Multi-App Budget Check: %s\n\n", status)
	fmt.Fprintln(table, "App\tInitial JS\tTotal JS\tBudget Status\tViolations")
	fmt.Fprintln(table, "---\t----------\t--------\t-------------\t----------")
	for _, s := range res.Summary {
		appStatus := "PASS"
		if !s.Passed {
			appStatus = "FAIL"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%d\n", s.Name, formatBytes(s.InitialJS), formatBytes(s.TotalJS), appStatus, s.Violations)
	}
	_ = table.Flush()

	if !res.Passed {
		text.WriteString("\nViolations:\n")
		for _, s := range res.Summary {
			if !s.Passed {
				proj := res.Projects[s.Name]
				for _, v := range proj.Violations {
					text.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, v.Message))
				}
			}
		}
	}
	_, err := io.WriteString(w, text.String())
	return err
}

// MultiAppCheckMarkdown renders GitHub Flavored Markdown for multi-app budget checks.
func MultiAppCheckMarkdown(w io.Writer, res budget.MultiAppCheckResult) error {
	var sb strings.Builder
	statusBadge := "✅ Passed"
	if !res.Passed {
		statusBadge = "❌ Failed"
	}
	fmt.Fprintf(&sb, "## 📦 BundleRadar Multi-App Budget Report — %s\n\n", statusBadge)
	sb.WriteString("| App | Status | Initial JS | Total JS | Violations |\n")
	sb.WriteString("| :--- | :---: | :---: | :---: | :---: |\n")
	for _, s := range res.Summary {
		appStatus := "✅ Pass"
		if !s.Passed {
			appStatus = "❌ Fail"
		}
		fmt.Fprintf(&sb, "| **`%s`** | %s | `%s` | `%s` | %d |\n", s.Name, appStatus, formatBytes(s.InitialJS), formatBytes(s.TotalJS), s.Violations)
	}
	sb.WriteString("\n")

	if !res.Passed {
		for _, s := range res.Summary {
			if !s.Passed {
				proj := res.Projects[s.Name]
				fmt.Fprintf(&sb, "<details>\n<summary>🔍 <b>%s Violations</b></summary>\n\n", s.Name)
				for _, v := range proj.Violations {
					fmt.Fprintf(&sb, "- **%s**: %s\n", v.Metric, v.Message)
				}
				sb.WriteString("</details>\n\n")
			}
		}
	}
	_, err := io.WriteString(w, sb.String())
	return err
}

// MultiAppCheckGitHubPR renders sticky comment Markdown for multi-app budget checks.
func MultiAppCheckGitHubPR(w io.Writer, res budget.MultiAppCheckResult) error {
	var sb strings.Builder
	sb.WriteString("<!-- bundleradar-multi-project-report -->\n")
	if err := MultiAppCheckMarkdown(&sb, res); err != nil {
		return err
	}
	_, err := io.WriteString(w, sb.String())
	return err
}
