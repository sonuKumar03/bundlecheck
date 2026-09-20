package report

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/sonuKumar03/bundlecheck/internal/budget"
	"github.com/sonuKumar03/bundlecheck/internal/comparison"
)

// IsGitHubPRFormat returns true if format represents a GitHub PR comment format.
func IsGitHubPRFormat(format string) bool {
	f := strings.ToLower(strings.TrimSpace(format))
	return f == "github-pr" || f == "pr" || f == "sticky-pr"
}

func renderDiffBar(delta, before int64) string {
	if delta == 0 || before <= 0 {
		return "<code>[──────────]</code> ⚪ 0.0%"
	}
	pct := (float64(delta) / float64(before)) * 100.0
	fill := int(math.Min(10, math.Max(1, math.Abs(pct)/2.0)))
	blocks := strings.Repeat("█", fill) + strings.Repeat("░", 10-fill)
	if delta < 0 {
		return fmt.Sprintf("<code>[%s]</code> 🟢 %.2f%%", blocks, pct)
	}
	return fmt.Sprintf("<code>[%s]</code> ⚠️ +%.2f%%", blocks, pct)
}

func renderPRMetricRow(sb *strings.Builder, name string, before, after, delta int64) {
	pctStr := "0.0%"
	if before > 0 {
		pct := (float64(delta) / float64(before)) * 100.0
		if delta > 0 {
			pctStr = fmt.Sprintf("+%.2f%%", pct)
		} else {
			pctStr = fmt.Sprintf("%.2f%%", pct)
		}
	}
	bar := renderDiffBar(delta, before)
	fmt.Fprintf(sb, "| **%s** | `%s` | `%s` | **%s** | `%s` | %s |\n",
		name, formatBytes(before), formatBytes(after), formatDelta(delta), pctStr, bar)
}

// ComparisonGitHubPR renders before/after comparison as an executive GitHub PR sticky comment.
func ComparisonGitHubPR(w io.Writer, r *comparison.Result, budgetCheck budget.CheckResult, opts TextOptions) error {
	var sb strings.Builder
	sb.WriteString("<!-- bundlecheck-comment -->\n")

	initialDelta := r.Summary.Delta.InitialJS
	badge := "⚪ Size Unchanged"
	if !budgetCheck.Passed {
		badge = fmt.Sprintf("❌ Budget Exceeded (%d Violations)", len(budgetCheck.Violations))
	} else if initialDelta < 0 {
		badge = fmt.Sprintf("🟢 Size Reduced (%s)", formatDelta(initialDelta))
	} else if initialDelta > 0 {
		badge = fmt.Sprintf("⚠️ Size Increased (%s)", formatDelta(initialDelta))
	}

	fmt.Fprintf(&sb, "## 📦 BundleCheck PR Report — %s\n\n", badge)

	if !budgetCheck.Passed && len(budgetCheck.Violations) > 0 {
		sb.WriteString("### ❌ Budget Violations\n\n")
		sb.WriteString("| Metric | Actual | Limit | Details |\n")
		sb.WriteString("| :--- | :---: | :---: | :--- |\n")
		for _, v := range budgetCheck.Violations {
			fmt.Fprintf(&sb, "| **`%s`** | `%s` | `%s` | %s |\n", v.Metric, formatBytes(v.Actual), formatBytes(v.Limit), v.Message)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("| Metric | Before | After | Delta | % Change | Visual Diff |\n")
	sb.WriteString("| :--- | :---: | :---: | :---: | :---: | :--- |\n")

	renderPRMetricRow(&sb, "Initial JS", r.Summary.Before.InitialJS, r.Summary.After.InitialJS, r.Summary.Delta.InitialJS)
	renderPRMetricRow(&sb, "Lazy JS", r.Summary.Before.LazyJS, r.Summary.After.LazyJS, r.Summary.Delta.LazyJS)
	renderPRMetricRow(&sb, "Total JS", r.Summary.Before.TotalJS, r.Summary.After.TotalJS, r.Summary.Delta.TotalJS)
	sb.WriteString("\n")

	// Package changes table inside collapsible details
	var changedRows []string
	var unchangedRows []string

	numAdded := 0
	numRemoved := 0
	numChanged := 0

	shown := 0
	limit := opts.Top
	if limit <= 0 {
		limit = 10
	}
	if opts.All {
		limit = len(r.Packages)
	}

	for _, p := range r.Packages {
		if opts.Filter != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(opts.Filter)) {
			continue
		}

		statusIcon := "🔄 Changed"
		switch p.Status {
		case "added":
			statusIcon = "➕ Added"
			numAdded++
		case "removed":
			statusIcon = "➖ Removed"
			numRemoved++
		case "unchanged":
			statusIcon = "⚪ Unchanged"
		default:
			numChanged++
		}

		row := fmt.Sprintf("| `%s` | %s | `%s` | `%s` |",
			p.Name, statusIcon, formatDelta(p.Delta.InitialBytes), formatDelta(p.Delta.TotalBytes))

		if p.Status == "unchanged" {
			unchangedRows = append(unchangedRows, row)
		} else {
			if shown < limit {
				changedRows = append(changedRows, row)
				shown++
			}
		}
	}

	if len(changedRows) > 0 {
		var breakdown []string
		if numChanged > 0 {
			breakdown = append(breakdown, fmt.Sprintf("%d modified", numChanged))
		}
		if numRemoved > 0 {
			breakdown = append(breakdown, fmt.Sprintf("%d removed", numRemoved))
		}
		if numAdded > 0 {
			breakdown = append(breakdown, fmt.Sprintf("%d added", numAdded))
		}
		summaryLabel := fmt.Sprintf("📦 Changed Packages (%s)", strings.Join(breakdown, ", "))
		if len(breakdown) == 0 {
			summaryLabel = fmt.Sprintf("📦 Changed Packages (%d total)", len(changedRows))
		}

		fmt.Fprintf(&sb, "<details>\n<summary><b>%s</b></summary>\n\n", summaryLabel)
		sb.WriteString("| Package | Status | Initial Delta | Total Delta |\n")
		sb.WriteString("| :--- | :---: | :---: | :---: |\n")
		for _, row := range changedRows {
			sb.WriteString(row + "\n")
		}
		sb.WriteString("\n</details>\n\n")
	} else {
		sb.WriteString("*(no package changes detected)*\n\n")
	}

	if len(unchangedRows) > 0 {
		fmt.Fprintf(&sb, "<details>\n<summary>⚪ %d Unchanged Packages</summary>\n\n", len(unchangedRows))
		sb.WriteString("| Package | Status | Initial Delta | Total Delta |\n")
		sb.WriteString("| :--- | :---: | :---: | :---: |\n")
		for _, row := range unchangedRows {
			sb.WriteString(row + "\n")
		}
		sb.WriteString("\n</details>\n\n")
	}

	sb.WriteString("<sub>Generated by <a href=\"https://github.com/sonuKumar03/bundlecheck\">bundlecheck</a></sub>\n")

	_, err := io.WriteString(w, sb.String())
	return err
}

// CheckGitHubPR renders budget check results with a sticky comment header.
func CheckGitHubPR(w io.Writer, res budget.CheckResult) error {
	var sb strings.Builder
	sb.WriteString("<!-- bundlecheck-comment -->\n")
	if err := CheckMarkdown(&sb, res); err != nil {
		return err
	}
	_, err := io.WriteString(w, sb.String())
	return err
}
