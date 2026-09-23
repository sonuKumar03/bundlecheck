package reporters

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/internal/core/diff"
	"github.com/sonuKumar03/bundleradar/internal/core/policy"
)

type MarkdownReporter struct{}

func (r *MarkdownReporter) Format() string {
	return "markdown"
}

func (r *MarkdownReporter) Render(ctx context.Context, w io.Writer, data any) error {
	var sb strings.Builder

	switch v := data.(type) {
	case *core.Bundle:
		sb.WriteString("## ⚡ BundleRadar Bundle Scan\n\n")
		sb.WriteString("| Entrypoint | Initial JS | Initial Gzip | Async JS | Chunks |\n")
		sb.WriteString("| :--- | :--- | :--- | :--- | :---: |\n")
		for name, ep := range v.Entrypoints {
			fmt.Fprintf(&sb, "| **`%s`** | `%s` | `%s` | `%s` | %d |\n",
				name, formatBytes(ep.InitialBytes), formatBytes(ep.InitialGzipBytes), formatBytes(ep.AsyncBytes), len(ep.ChunkIDs))
		}
		sb.WriteString("\n")

		top := v.TopPackages(10)
		if len(top) > 0 {
			sb.WriteString("### 📦 Top Contributing NPM Packages\n\n")
			sb.WriteString("| Package | Size | Est. Gzip | Modules |\n")
			sb.WriteString("| :--- | :--- | :--- | :---: |\n")
			for _, pkg := range top {
				fmt.Fprintf(&sb, "| `%s` | `%s` | `%s` | %d |\n",
					pkg.Name, formatBytes(pkg.SizeBytes), formatBytes(pkg.GzipBytes), pkg.ModuleCount)
			}
			sb.WriteString("\n")
		}

	case *diff.BundleDiff:
		sb.WriteString("## ⚡ BundleRadar Comparison & Diff\n\n")
		sb.WriteString("| Entrypoint | Initial Delta | Gzip Delta | Async Delta |\n")
		sb.WriteString("| :--- | :--- | :--- | :--- |\n")
		for name, ep := range v.Entrypoints {
			initSign := "+"
			if ep.InitialDelta < 0 {
				initSign = ""
			}
			fmt.Fprintf(&sb, "| **`%s`** | `%s%s` | `+%s` | `+%s` |\n",
				name, initSign, formatBytes(ep.InitialDelta), formatBytes(ep.InitialGzipDelta), formatBytes(ep.AsyncDelta))
		}
		sb.WriteString("\n")

		if v.MicroDriftBytes > 0 {
			fmt.Fprintf(&sb, "> ℹ️ **Micro-drift**: %s collapsed across sub-threshold module updates.\n\n", formatBytes(v.MicroDriftBytes))
		}

	case policy.EvaluationResult:
		if v.Passed {
			sb.WriteString("## ⚡ BundleRadar Gate: ✅ All Size Budgets Passed\n\n")
			sb.WriteString("All bundle size limits and architectural policies are satisfied.\n\n")
		} else {
			sb.WriteString("## ⚡ BundleRadar Gate: ❌ Policy Check Failed\n\n")
			sb.WriteString("| Rule | Severity | Violation Message |\n")
			sb.WriteString("| :--- | :--- | :--- |\n")
			for _, viol := range v.Violations {
				fmt.Fprintf(&sb, "| **`%s`** | `%s` | %s |\n", viol.Rule, viol.Severity, viol.Message)
			}
			sb.WriteString("\n")
		}

	default:
		return fmt.Errorf("markdown reporter: unsupported data type %T", data)
	}

	_, err := io.WriteString(w, sb.String())
	return err
}
