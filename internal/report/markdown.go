// Package report renders structured results as CLI text, JSON, and GitHub Flavored Markdown.
package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/sonuKumar03/bundlecheck/internal/advisor"
	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/budget"
	"github.com/sonuKumar03/bundlecheck/internal/comparison"
)

// IsMarkdownFormat returns true if format is "markdown" or "md".
func IsMarkdownFormat(format string) bool {
	f := strings.ToLower(strings.TrimSpace(format))
	return f == "markdown" || f == "md"
}

// SummaryMarkdown renders bundle summary as a GitHub Flavored Markdown table.
func SummaryMarkdown(w io.Writer, r *analysis.AnalysisResult, opts TextOptions) error {
	var sb strings.Builder

	sb.WriteString("## 📦 Angular Bundle Summary\n\n")

	if opts.Gzip && r.Summary.TotalGzipJS > 0 {
		sb.WriteString("| Category | Size | Gzip |\n")
		sb.WriteString("| :--- | :--- | :--- |\n")
		fmt.Fprintf(&sb, "| **Initial JS** | `%s` | `%s` |\n", formatBytes(r.Summary.InitialJS), formatBytes(r.Summary.InitialGzipJS))
		fmt.Fprintf(&sb, "| **Lazy JS** | `%s` | `%s` |\n", formatBytes(r.Summary.LazyJS), formatBytes(r.Summary.LazyGzipJS))
		fmt.Fprintf(&sb, "| **Total JS** | `%s` | `%s` |\n\n", formatBytes(r.Summary.TotalJS), formatBytes(r.Summary.TotalGzipJS))
	} else {
		sb.WriteString("| Category | Size |\n")
		sb.WriteString("| :--- | :--- |\n")
		fmt.Fprintf(&sb, "| **Initial JS** | `%s` |\n", formatBytes(r.Summary.InitialJS))
		fmt.Fprintf(&sb, "| **Lazy JS** | `%s` |\n", formatBytes(r.Summary.LazyJS))
		fmt.Fprintf(&sb, "| **Total JS** | `%s` |\n\n", formatBytes(r.Summary.TotalJS))
	}

	header := "### Top NPM Contributors"
	if opts.Filter != "" {
		header = fmt.Sprintf("### Top NPM Contributors matching %q", opts.Filter)
	}
	sb.WriteString(header + "\n\n")

	shown := 0
	limit := opts.Top
	if limit <= 0 {
		limit = 10
	}
	if opts.All {
		limit = len(r.Packages)
	}

	hasRows := false
	for _, p := range r.Packages {
		if p.InitialBytes == 0 {
			continue
		}
		if opts.Filter != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(opts.Filter)) {
			continue
		}

		if !hasRows {
			if opts.Gzip && p.InitialGzipBytes > 0 {
				sb.WriteString("| Package | Initial Size | Share | Gzip |\n")
				sb.WriteString("| :--- | :--- | :--- | :--- |\n")
			} else {
				sb.WriteString("| Package | Initial Size | Share |\n")
				sb.WriteString("| :--- | :--- | :--- |\n")
			}
			hasRows = true
		}

		pct := "0.0%"
		if r.Summary.InitialJS > 0 {
			share := (float64(p.InitialBytes) / float64(r.Summary.InitialJS)) * 100
			pct = fmt.Sprintf("%.1f%%", share)
		}

		if opts.Gzip && p.InitialGzipBytes > 0 {
			fmt.Fprintf(&sb, "| `%s` | `%s` | %s | `%s` |\n", p.Name, formatBytes(p.InitialBytes), pct, formatBytes(p.InitialGzipBytes))
		} else {
			fmt.Fprintf(&sb, "| `%s` | `%s` | %s |\n", p.Name, formatBytes(p.InitialBytes), pct)
		}

		shown++
		if shown >= limit {
			break
		}
	}

	if !hasRows {
		sb.WriteString("*(none)*\n")
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

// ComparisonMarkdown renders before/after comparison as a formatted PR summary table.
func ComparisonMarkdown(w io.Writer, r *comparison.Result, opts TextOptions) error {
	var sb strings.Builder

	delta := r.Summary.Delta.InitialJS
	badge := "🟢 Neutral"
	if delta < 0 {
		badge = fmt.Sprintf("✅ Reduced (%s)", formatDelta(delta))
	} else if delta > 0 {
		badge = fmt.Sprintf("⚠️ Increased (%s)", formatDelta(delta))
	}

	fmt.Fprintf(&sb, "## 📊 Angular Bundle Comparison — %s\n\n", badge)

	sb.WriteString("| Category | Before | After | Delta | Status |\n")
	sb.WriteString("| :--- | :---: | :---: | :---: | :---: |\n")

	renderMetricRow(&sb, "Initial JS", r.Summary.Before.InitialJS, r.Summary.After.InitialJS, r.Summary.Delta.InitialJS)
	renderMetricRow(&sb, "Lazy JS", r.Summary.Before.LazyJS, r.Summary.After.LazyJS, r.Summary.Delta.LazyJS)
	renderMetricRow(&sb, "Total JS", r.Summary.Before.TotalJS, r.Summary.After.TotalJS, r.Summary.Delta.TotalJS)
	sb.WriteString("\n")

	if len(r.Findings) > 0 {
		sb.WriteString("### 🔎 Regression Explanation\n\n")
		for _, f := range r.Findings {
			chunkInfo := ""
			if len(f.Chunks) > 0 {
				chunkInfo = fmt.Sprintf(" → emitted in `%s`", strings.Join(f.Chunks, "`, `"))
			}
			reasonInfo := ""
			if f.Reason != "" {
				reasonInfo = fmt.Sprintf(" *(%s)*", f.Reason)
			}
			icon := "📦"
			switch f.Kind {
			case "source":
				icon = "📁"
			case "unattributed":
				icon = "⚪"
			case "package":
				icon = "📦"
			}
			fmt.Fprintf(&sb, "- %s **`%s`** (`%s`)%s%s\n", icon, f.Name, formatDelta(f.DeltaBytes), chunkInfo, reasonInfo)
			if len(f.TracePath) > 0 {
				sb.WriteString("  - **Import path:** " + FormatTracePath(f.TracePath) + "\n")
			}
		}
		sb.WriteString("\n")
	}

	// Package changes table
	sb.WriteString("### Changed Packages\n\n")

	shown := 0
	limit := opts.Top
	if limit <= 0 {
		limit = 10
	}
	if opts.All {
		limit = len(r.Packages)
	}

	var changedRows []string
	var unchangedRows []string

	for _, p := range r.Packages {
		if opts.Filter != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(opts.Filter)) {
			continue
		}

		statusIcon := "🔄 Changed"
		switch p.Status {
		case "added":
			statusIcon = "➕ Added"
		case "removed":
			statusIcon = "➖ Removed"
		case "unchanged":
			statusIcon = "⚪ Unchanged"
		}

		row := fmt.Sprintf("| `%s` | %s | `%s` | `%s` | `%s` |",
			p.Name, statusIcon, formatDelta(p.Delta.InitialBytes), formatDelta(p.Delta.LazyBytes), formatDelta(p.Delta.TotalBytes))

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
		sb.WriteString("| Package | Status | Initial Delta | Lazy Delta | Total Delta |\n")
		sb.WriteString("| :--- | :---: | :---: | :---: | :---: |\n")
		for _, row := range changedRows {
			sb.WriteString(row + "\n")
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("*(no package changes detected)*\n\n")
	}

	if len(unchangedRows) > 0 {
		fmt.Fprintf(&sb, "<details><summary>⚪ %d Unchanged Packages</summary>\n\n", len(unchangedRows))
		sb.WriteString("| Package | Status | Initial Delta | Lazy Delta | Total Delta |\n")
		sb.WriteString("| :--- | :---: | :---: | :---: | :---: |\n")
		for _, row := range unchangedRows {
			sb.WriteString(row + "\n")
		}
		sb.WriteString("\n</details>\n")
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

func renderMetricRow(sb *strings.Builder, name string, before, after, delta int64) {
	status := "⚪ Neutral"
	if delta < 0 {
		status = "✅ Reduced"
	} else if delta > 0 {
		status = "⚠️ Increased"
	}
	fmt.Fprintf(sb, "| **%s** | `%s` | `%s` | **%s** | %s |\n", name, formatBytes(before), formatBytes(after), formatDelta(delta), status)
}

// SuggestMarkdown renders culprit analysis in GitHub Flavored Markdown.
func SuggestMarkdown(w io.Writer, r *advisor.AdvisorResult, showGzip bool) error {
	var sb strings.Builder

	sb.WriteString("## 🔍 Initial Bundle Culprits & Contributors\n\n")

	if len(r.Suggestions) == 0 {
		sb.WriteString("✅ **No major culprits detected.** Your initial bundle structure looks optimal!\n")
		_, err := io.WriteString(w, sb.String())
		return err
	}

	savingsStr := formatBytes(r.TotalPotentialSavings)
	fmt.Fprintf(&sb, "> **Initial JS Impact:** ~`%s` across %d detected contributor(s)\n\n", savingsStr, len(r.Suggestions))

	for i, s := range r.Suggestions {
		badge := "🔵 LOW"
		switch s.Severity {
		case "HIGH":
			badge = "🔴 HIGH"
		case "MEDIUM":
			badge = "🟡 MEDIUM"
		}

		savingsDisplay := formatBytes(s.Savings)
		if showGzip && s.SavingsGzip > 0 {
			savingsDisplay += fmt.Sprintf(" (~%s gzip)", formatBytes(s.SavingsGzip))
		}

		fmt.Fprintf(&sb, "### %d. %s: %s\n", i+1, badge, s.Title)
		fmt.Fprintf(&sb, "- **Initial JS Impact:** ~`%s`\n", savingsDisplay)
		if s.File != "" {
			fmt.Fprintf(&sb, "- **Culprit / Importer:** `%s`\n", s.File)
		}
		fmt.Fprintf(&sb, "- **Reason:** %s\n", s.Description)
		if s.Action != "" {
			fmt.Fprintf(&sb, "- **Recommended Action:**\n  ```ts\n  %s\n  ```\n", s.Action)
		}
		sb.WriteString("\n")
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

// CheckMarkdown renders budget check results in Markdown.
func CheckMarkdown(w io.Writer, res budget.CheckResult) error {
	var sb strings.Builder

	if res.Passed {
		sb.WriteString("## ✅ Angular Bundle Budget Check: PASSED\n\n")
		sb.WriteString("All configured bundle size budgets and delta thresholds passed successfully.\n")
	} else {
		fmt.Fprintf(&sb, "## ❌ Angular Bundle Budget Check: FAILED (%d Violations)\n\n", len(res.Violations))
		sb.WriteString("| Metric | Actual Size | Allowed Limit | Details |\n")
		sb.WriteString("| :--- | :---: | :---: | :--- |\n")
		for _, v := range res.Violations {
			fmt.Fprintf(&sb, "| **`%s`** | `%s` | `%s` | %s |\n", v.Metric, formatBytes(v.Actual), formatBytes(v.Limit), v.Message)
		}
		sb.WriteString("\n")
	}

	_, err := io.WriteString(w, sb.String())
	return err
}
