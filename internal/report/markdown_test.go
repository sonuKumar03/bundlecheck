package report_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/advisor"
	"github.com/sonuKumar03/bundleradar/internal/analysis"
	"github.com/sonuKumar03/bundleradar/internal/budget"
	"github.com/sonuKumar03/bundleradar/internal/comparison"
	"github.com/sonuKumar03/bundleradar/internal/report"
	"github.com/sonuKumar03/bundleradar/internal/snapshot"
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
		"- 📦 **`chart.js`** (`+31 KB`) → emitted in `dist/browser/main.js`",
		"- **Import path:** `src/main.ts` → `node_modules/chart.js/auto.js`",
		"- ⚪ **`(unattributed)`** (`+11 KB`) *(Growth in application sources)*",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in markdown output:\n%s", want, out)
		}
	}
}

func TestComparisonMarkdown_SourceFinding(t *testing.T) {
	comp := &comparison.Result{
		Summary: comparison.SummaryChange{
			Delta: snapshot.Totals{InitialJS: 20480},
		},
		Findings: []comparison.Finding{
			{
				Name:       "projects/movies/src/app/pages/movie-detail-page",
				DeltaBytes: 20480,
				Kind:       "source",
				Chunks:     []string{"main.js"},
				Reason:     "Application component moved into initial bundle",
			},
		},
	}
	var buf bytes.Buffer
	err := report.ComparisonMarkdown(&buf, comp, report.TextOptions{Top: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "movie-detail-page") {
		t.Errorf("expected movie-detail-page in markdown output:\n%s", out)
	}
	if !strings.Contains(out, "📁") {
		t.Errorf("expected 📁 source indicator in markdown output:\n%s", out)
	}
	wantLine := "- 📁 **`projects/movies/src/app/pages/movie-detail-page`** (`+20 KB`) → emitted in `main.js` *(Application component moved into initial bundle)*"
	if !strings.Contains(out, wantLine) {
		t.Errorf("expected %q in markdown output:\n%s", wantLine, out)
	}
}

func TestComparisonMarkdown_ImportPath(t *testing.T) {
	comp := &comparison.Result{
		Summary: comparison.SummaryChange{
			Delta: snapshot.Totals{InitialJS: 50 * 1024},
		},
		Findings: []comparison.Finding{
			{
				Name:       "moment-timezone",
				DeltaBytes: 50 * 1024,
				Kind:       "package",
				Chunks:     []string{"dist/browser/main.js"},
				TracePath: []string{
					"apps/portal/src/main.ts",
					"apps/portal/src/app/app.config.ts",
					"apps/portal/src/app/app.module.ts",
					"libs/timezone-scheduler/src/index.ts",
					"node_modules/moment-timezone/index.js",
				},
			},
		},
	}

	var buf bytes.Buffer
	err := report.ComparisonMarkdown(&buf, comp, report.TextOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	want := "- **Import path:** `apps/portal/src/main.ts` → `apps/portal/src/app/app.module.ts` → `libs/timezone-scheduler/src/index.ts` → `node_modules/moment-timezone/index.js`"
	if !strings.Contains(out, want) {
		t.Errorf("expected formatted import path in markdown:\n  want: %s\n  got:\n%s", want, out)
	}
	if strings.Contains(out, "app.config.ts") {
		t.Errorf("expected redundant intermediate hop 'app.config.ts' to be omitted, got:\n%s", out)
	}
}

func TestComparisonMarkdown_ProjectTitle(t *testing.T) {
	comp := &comparison.Result{
		Summary: comparison.SummaryChange{
			Delta: snapshot.Totals{InitialJS: 0},
		},
	}
	var buf bytes.Buffer
	if err := report.ComparisonMarkdown(&buf, comp, report.TextOptions{Project: "portal"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "## 📊 Angular Bundle Comparison (`portal`) — ⚪ Neutral") {
		t.Errorf("expected project title in output:\n%s", out)
	}
}

func TestComparisonMarkdown_MicroDriftCollapsing(t *testing.T) {
	comp := &comparison.Result{
		Summary: comparison.SummaryChange{
			Delta: snapshot.Totals{InitialJS: 300 * 1024},
		},
		Findings: []comparison.Finding{
			{
				Name:       "three",
				DeltaBytes: 295 * 1024,
				Kind:       "package",
				Chunks:     []string{"main.js"},
			},
			{
				Name:       "lodash",
				DeltaBytes: 11,
				Kind:       "package",
				Chunks:     []string{"main.js"},
			},
			{
				Name:       "@angular/platform-browser",
				DeltaBytes: 2,
				Kind:       "package",
				Chunks:     []string{"main.js"},
			},
		},
	}

	var buf bytes.Buffer
	err := report.ComparisonMarkdown(&buf, comp, report.TextOptions{DriftThreshold: 1024})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	// Major finding should be under Regression Explanation
	if !strings.Contains(out, "### 🔎 Regression Explanation") {
		t.Fatalf("missing Regression Explanation:\n%s", out)
	}
	if !strings.Contains(out, "- 📦 **`three`** (`+295 KB`)") {
		t.Errorf("expected three in major findings:\n%s", out)
	}

	// Minor findings should be collapsed
	if !strings.Contains(out, "<summary>⚪ 2 Minor Variations (< 1 KB)</summary>") {
		t.Errorf("expected 2 Minor Variations collapsed section:\n%s", out)
	}
	if !strings.Contains(out, "- 📦 **`lodash`** (`+11 B`)") {
		t.Errorf("expected lodash in collapsed minor findings:\n%s", out)
	}
	if !strings.Contains(out, "- 📦 **`@angular/platform-browser`** (`+2 B`)") {
		t.Errorf("expected @angular/platform-browser in collapsed minor findings:\n%s", out)
	}
}
