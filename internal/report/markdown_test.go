package report_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/advisor"
	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/budget"
	"github.com/sonuKumar03/bundlecheck/internal/comparison"
	"github.com/sonuKumar03/bundlecheck/internal/report"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

func TestSummaryMarkdown(t *testing.T) {
	res := &analysis.AnalysisResult{
		Summary: snapshot.Totals{
			InitialJS: 160 * 1024,
			LazyJS:    300 * 1024,
			TotalJS:   460 * 1024,
		},
		Packages: []snapshot.Package{
			{
				Name:         "lodash",
				InitialBytes: 50 * 1024,
			},
		},
	}

	var buf bytes.Buffer
	err := report.SummaryMarkdown(&buf, res, report.TextOptions{Top: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "## 📦 Angular Bundle Summary") || !strings.Contains(out, "| **Initial JS** | `160 KB` |") {
		t.Errorf("markdown table format mismatch: %s", out)
	}
}

func TestComparisonMarkdown(t *testing.T) {
	comp := &comparison.Result{
		Summary: comparison.SummaryChange{
			Before: snapshot.Totals{InitialJS: 160 * 1024, TotalJS: 460 * 1024},
			After:  snapshot.Totals{InitialJS: 140 * 1024, TotalJS: 460 * 1024},
			Delta:  snapshot.Totals{InitialJS: -20 * 1024, TotalJS: 0},
		},
		Packages: []comparison.PackageChange{
			{
				Name:   "pdfjs-dist",
				Status: "changed",
				Delta:  comparison.Bytes{InitialBytes: -20 * 1024},
			},
		},
	}

	var buf bytes.Buffer
	err := report.ComparisonMarkdown(&buf, comp, report.TextOptions{Top: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "✅ Reduced") || !strings.Contains(out, "pdfjs-dist") {
		t.Errorf("expected comparison markdown badges and rows: %s", out)
	}
}

func TestSuggestMarkdown(t *testing.T) {
	adv := &advisor.AdvisorResult{
		TotalPotentialSavings: 50 * 1024,
		Suggestions: []advisor.Suggestion{
			{
				Rule:        "heavy-initial-package",
				Severity:    "HIGH",
				Title:       "Move 'pdfjs-dist' behind dynamic import",
				Savings:     50 * 1024,
				Description: "Heavy utility in initial bundle",
				Action:      "const lib = await import('pdfjs-dist')",
			},
		},
	}

	var buf bytes.Buffer
	err := report.SuggestMarkdown(&buf, adv, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "🔴 HIGH") || !strings.Contains(out, "pdfjs-dist") {
		t.Errorf("unexpected suggest markdown output: %s", out)
	}
}

func TestCheckMarkdown(t *testing.T) {
	resPass := budget.CheckResult{Passed: true}
	var bufPass bytes.Buffer
	_ = report.CheckMarkdown(&bufPass, resPass)
	if !strings.Contains(bufPass.String(), "PASSED") {
		t.Errorf("expected pass markdown, got: %s", bufPass.String())
	}

	resFail := budget.CheckResult{
		Passed: false,
		Violations: []budget.Violation{
			{
				Metric:  "initialJs",
				Actual:  2000,
				Limit:   1000,
				Message: "exceeded budget",
			},
		},
	}
	var bufFail bytes.Buffer
	_ = report.CheckMarkdown(&bufFail, resFail)
	if !strings.Contains(bufFail.String(), "FAILED") || !strings.Contains(bufFail.String(), "initialJs") {
		t.Errorf("expected fail markdown, got: %s", bufFail.String())
	}
}

func TestComparisonMarkdownWithFindings(t *testing.T) {
	comp := &comparison.Result{
		Summary: comparison.SummaryChange{
			Before: snapshot.Totals{InitialJS: 100 * 1024, TotalJS: 100 * 1024},
			After:  snapshot.Totals{InitialJS: 142 * 1024, TotalJS: 142 * 1024},
			Delta:  snapshot.Totals{InitialJS: 42 * 1024, TotalJS: 42 * 1024},
		},
		Findings: []comparison.Finding{
			{
				Name:       "chart.js",
				DeltaBytes: 31 * 1024,
				Kind:       "package",
				Chunks:     []string{"dist/browser/main.js"},
				TracePath:  []string{"src/main.ts", "node_modules/chart.js/auto.js"},
			},
			{
				Name:       "(unattributed)",
				DeltaBytes: 11 * 1024,
				Kind:       "unattributed",
				Reason:     "Growth in application sources",
			},
		},
	}

	var buf bytes.Buffer
	err := report.ComparisonMarkdown(&buf, comp, report.TextOptions{Top: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"### 🔎 Regression Explanation",
		"- **`chart.js`** (`+31 KB`) → emitted in `dist/browser/main.js`",
		"- **Import path:** `src/main.ts` → `node_modules/chart.js/auto.js`",
		"- **`(unattributed)`** (`+11 KB`) *(Growth in application sources)*",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in markdown output:\n%s", want, out)
		}
	}
}
