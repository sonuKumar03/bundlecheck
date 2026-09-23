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

type TerminalReporter struct{}

func (r *TerminalReporter) Format() string {
	return "terminal"
}

func (r *TerminalReporter) Render(ctx context.Context, w io.Writer, data any) error {
	var sb strings.Builder

	switch v := data.(type) {
	case *core.Bundle:
		sb.WriteString("\n⚡ BUNDLERADAR BUNDLE SCAN\n")
		sb.WriteString("-------------------------------------------------------------\n")
		for name, ep := range v.Entrypoints {
			fmt.Fprintf(&sb, "Entrypoint %q:\n", name)
			fmt.Fprintf(&sb, "  Initial JavaScript:   %s [~%s gzip]\n", formatBytes(ep.InitialBytes), formatBytes(ep.InitialGzipBytes))
			fmt.Fprintf(&sb, "  Async / Lazy Chunks:  %s (%d chunks)\n", formatBytes(ep.AsyncBytes), len(ep.ChunkIDs))
		}
		sb.WriteString("\n")

		top := v.TopPackages(5)
		if len(top) > 0 {
			sb.WriteString("TOP CONTRIBUTING NPM PACKAGES\n")
			sb.WriteString("-------------------------------------------------------------\n")
			for i, pkg := range top {
				fmt.Fprintf(&sb, "%d. %-20s %10s  [~%s gzip] (%d modules)\n",
					i+1, pkg.Name, formatBytes(pkg.SizeBytes), formatBytes(pkg.GzipBytes), pkg.ModuleCount)
			}
			sb.WriteString("\n")
		}

	case *diff.BundleDiff:
		sb.WriteString("\n⚡ BUNDLERADAR COMPARISON & DIFF\n")
		sb.WriteString("-------------------------------------------------------------\n")
		for name, ep := range v.Entrypoints {
			sign := "+"
			if ep.InitialDelta < 0 {
				sign = ""
			}
			fmt.Fprintf(&sb, "Entrypoint %q Initial Delta: %s%s [~+%s gzip]\n",
				name, sign, formatBytes(ep.InitialDelta), formatBytes(ep.InitialGzipDelta))
		}
		if v.MicroDriftBytes > 0 {
			fmt.Fprintf(&sb, "Micro-drift: %s across sub-threshold updates\n", formatBytes(v.MicroDriftBytes))
		}
		sb.WriteString("\n")

	case policy.EvaluationResult:
		if v.Passed {
			sb.WriteString("\n⚡ BUNDLERADAR GATE: ✅ PASSED\n")
			sb.WriteString("All bundle size limits and architectural policies are satisfied.\n\n")
		} else {
			sb.WriteString("\n⚡ BUNDLERADAR GATE: ❌ POLICY VIOLATION\n")
			sb.WriteString("-------------------------------------------------------------\n")
			for _, viol := range v.Violations {
				fmt.Fprintf(&sb, "[!] %s: %s\n", viol.Rule, viol.Message)
			}
			sb.WriteString("\n")
		}

	default:
		return fmt.Errorf("terminal reporter: unsupported data type %T", data)
	}

	_, err := io.WriteString(w, sb.String())
	return err
}
