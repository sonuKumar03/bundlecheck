package report

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/budget"
	"github.com/sonuKumar03/bundlecheck/internal/comparison"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
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
	if delta < -MinorDriftThreshold {
		return fmt.Sprintf("<code>[%s]</code> 🟢 %.2f%%", blocks, pct)
	} else if delta > MinorDriftThreshold {
		return fmt.Sprintf("<code>[%s]</code> ⚠️ +%.2f%%", blocks, pct)
	}
	return fmt.Sprintf("<code>[──────────]</code> ⚪ +%.2f%%", pct)
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
	} else if initialDelta < -MinorDriftThreshold {
		badge = fmt.Sprintf("🟢 Size Reduced (%s)", formatDelta(initialDelta))
	} else if initialDelta > MinorDriftThreshold {
		badge = fmt.Sprintf("⚠️ Size Increased (%s)", formatDelta(initialDelta))
	} else if initialDelta != 0 {
		badge = fmt.Sprintf("⚪ Neutral (%s)", formatDelta(initialDelta))
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

	if len(r.Findings) > 0 {
		isMinor := initialDelta <= MinorDriftThreshold
		if isMinor {
			fmt.Fprintf(&sb, "<details>\n<summary>🔎 Minor Source Changes (%s)</summary>\n\n", formatDelta(initialDelta))
		} else {
			sb.WriteString("### 🔎 Regression Explanation\n\n")
		}
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
		if isMinor {
			sb.WriteString("\n</details>\n")
		}
		sb.WriteString("\n")
	}

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

// CompactTracePath reduces a long trace path (> 4 hops) to highlight essential hops:
// 1. Root application entrypoint (e.g. apps/portal/src/main.ts)
// 2. Application file bridging/importing the library or feature (e.g. apps/portal/src/app/app.module.ts)
// 3. Library boundary if present (e.g. libs/timezone-scheduler/src/index.ts)
// 4. Emitted dependency or target package (e.g. node_modules/moment-timezone/index.js)
// If the path is already concise (<= 4 hops), it is returned unmodified.
func CompactTracePath(path []string) []string {
	n := len(path)
	if n <= 4 {
		return path
	}

	// Categorize nodes:
	// 0: application code
	// 1: library code (workspace libs)
	// 2: external package (node_modules)
	categorize := func(p string) int {
		cleaned := snapshot.CleanPath(p)
		if analysis.IsPackage(cleaned) || strings.Contains(cleaned, "node_modules") {
			return 2
		}
		if strings.HasPrefix(cleaned, "libs/") || strings.Contains(cleaned, "/libs/") ||
			strings.HasPrefix(cleaned, "packages/") || strings.Contains(cleaned, "/packages/") {
			return 1
		}
		return 0
	}

	firstLib := -1
	firstPkg := -1
	for i, p := range path {
		cat := categorize(p)
		if cat == 1 && firstLib == -1 {
			firstLib = i
		}
		if cat == 2 && firstPkg == -1 {
			firstPkg = i
		}
	}

	var selectedIndices []int

	if firstLib != -1 {
		// Case A: Library boundary is present
		if firstLib > 1 {
			// Entrypoint, app bridging file, library boundary, target
			selectedIndices = []int{0, firstLib - 1, firstLib, n - 1}
		} else if firstLib == 1 {
			// Entrypoint immediately imports library
			boundaryTarget := n - 1
			if firstPkg > 0 {
				boundaryTarget = firstPkg
			}
			importer := boundaryTarget - 1
			if importer > firstLib {
				selectedIndices = []int{0, firstLib, importer, n - 1}
			} else {
				selectedIndices = []int{0, firstLib, boundaryTarget, n - 1}
			}
		}
	} else if firstPkg != -1 {
		// Case B: No library boundary, but external package is present
		importer := firstPkg - 1
		if importer <= 0 {
			importer = n - 2
		}

		// Look for an intermediate app bridge (module, component, routes) between root (0) and importer
		bridge := -1
		for i := importer - 1; i >= 1; i-- {
			lower := strings.ToLower(path[i])
			if strings.Contains(lower, "module") || strings.Contains(lower, "component") || strings.Contains(lower, "routes") {
				bridge = i
				break
			}
		}
		if bridge == -1 && importer > 1 {
			bridge = 1
		}

		if bridge != -1 && bridge != importer && bridge != 0 {
			selectedIndices = []int{0, bridge, importer, n - 1}
		} else {
			selectedIndices = []int{0, importer, n - 1}
		}
	} else {
		// Case C: All hops are internal application or library files
		importer := n - 2
		bridge := -1
		for i := importer - 1; i >= 1; i-- {
			lower := strings.ToLower(path[i])
			if strings.Contains(lower, "module") || strings.Contains(lower, "component") || strings.Contains(lower, "routes") {
				bridge = i
				break
			}
		}
		if bridge == -1 && importer > 1 {
			bridge = 1
		}

		if bridge != -1 && bridge != importer && bridge != 0 {
			selectedIndices = []int{0, bridge, importer, n - 1}
		} else {
			selectedIndices = []int{0, importer, n - 1}
		}
	}

	// Filter and deduplicate indices while preserving order
	var result []string
	seen := make(map[int]bool)
	for _, idx := range selectedIndices {
		if idx >= 0 && idx < n && !seen[idx] {
			seen[idx] = true
			result = append(result, path[idx])
		}
	}

	if len(result) < 2 {
		return path
	}

	return result
}

// FormatTracePath formats a trace path into a Markdown string with backticks and arrow separators,
// compacting long paths (> 4 hops) to highlight essential boundaries.
func FormatTracePath(path []string) string {
	if len(path) == 0 {
		return ""
	}
	compacted := CompactTracePath(path)
	return "`" + strings.Join(compacted, "` → `") + "`"
}

// formatTracePath is an unexported alias for FormatTracePath.
func formatTracePath(path []string) string {
	return FormatTracePath(path)
}

