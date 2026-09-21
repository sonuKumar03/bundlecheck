package report_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/budget"
	"github.com/sonuKumar03/bundlecheck/internal/comparison"
	"github.com/sonuKumar03/bundlecheck/internal/report"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

func TestIsGitHubPRFormat(t *testing.T) {
	tests := []struct {
		format string
		want   bool
	}{
		{"github-pr", true},
		{"GITHUB-PR", true},
		{"pr", true},
		{"sticky-pr", true},
		{"markdown", false},
		{"text", false},
		{"json", false},
	}

	for _, tt := range tests {
		if got := report.IsGitHubPRFormat(tt.format); got != tt.want {
			t.Errorf("IsGitHubPRFormat(%q) = %v, want %v", tt.format, got, tt.want)
		}
	}
}

func TestComparisonGitHubPR(t *testing.T) {
	comp := &comparison.Result{
		Summary: comparison.SummaryChange{
			Before: snapshot.Totals{
				InitialJS: 1000 * 1024,
				LazyJS:    500 * 1024,
				TotalJS:   1500 * 1024,
			},
			After: snapshot.Totals{
				InitialJS: 950 * 1024,
				LazyJS:    500 * 1024,
				TotalJS:   1450 * 1024,
			},
			Delta: snapshot.Totals{
				InitialJS: -50 * 1024,
				LazyJS:    0,
				TotalJS:   -50 * 1024,
			},
		},
		Packages: []comparison.PackageChange{
			{
				Name:   "lodash",
				Status: "removed",
				Delta: comparison.Bytes{
					InitialBytes: -50 * 1024,
					TotalBytes:   -50 * 1024,
				},
			},
			{
				Name:   "@angular/core",
				Status: "unchanged",
				Delta: comparison.Bytes{
					InitialBytes: 0,
					TotalBytes:   0,
				},
			},
		},
	}

	var buf bytes.Buffer
	opts := report.TextOptions{Top: 10}
	budgetCheck := budget.CheckResult{Passed: true}

	err := report.ComparisonGitHubPR(&buf, comp, budgetCheck, opts)
	if err != nil {
		t.Fatalf("ComparisonGitHubPR returned error: %v", err)
	}

	out := buf.String()

	// 1. Must contain sticky comment marker tag
	if !strings.Contains(out, "<!-- bundlecheck-comment -->") {
		t.Errorf("expected sticky comment marker <!-- bundlecheck-comment -->")
	}

	// 2. Must contain status badge
	if !strings.Contains(out, "🟢 Size Reduced") {
		t.Errorf("expected reduction headline, got:\n%s", out)
	}

	// 3. Must contain comparison table
	if !strings.Contains(out, "| **Initial JS** |") || !strings.Contains(out, "| **Lazy JS** |") {
		t.Errorf("expected metrics rows in table, got:\n%s", out)
	}

	// 4. Must contain visual diff bar
	if !strings.Contains(out, "<code>[") {
		t.Errorf("expected visual diff bar with <code>[, got:\n%s", out)
	}

	// 5. Must contain collapsible changed packages details
	if !strings.Contains(out, "<details>") || !strings.Contains(out, "lodash") {
		t.Errorf("expected collapsible package list containing lodash, got:\n%s", out)
	}

	// 6. Must not contain advisor recommendations
	if strings.Contains(out, "Optimization Opportunities") {
		t.Errorf("should not contain advisor recommendations")
	}
}

func TestComparisonGitHubPR_BudgetFailed(t *testing.T) {
	comp := &comparison.Result{
		Summary: comparison.SummaryChange{
			Before: snapshot.Totals{InitialJS: 1000},
			After:  snapshot.Totals{InitialJS: 2000},
			Delta:  snapshot.Totals{InitialJS: 1000},
		},
	}

	var buf bytes.Buffer
	opts := report.TextOptions{Top: 10}
	budgetCheck := budget.CheckResult{
		Passed: false,
		Violations: []budget.Violation{
			{
				Metric:  "initial-delta",
				Actual:  1000,
				Limit:   500,
				Message: "Initial JS delta +1000 B exceeds limit 500 B",
			},
		},
	}

	err := report.ComparisonGitHubPR(&buf, comp, budgetCheck, opts)
	if err != nil {
		t.Fatalf("ComparisonGitHubPR returned error: %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, "❌ Budget Exceeded") {
		t.Errorf("expected Budget Exceeded badge in output, got:\n%s", out)
	}
	if !strings.Contains(out, "initial-delta") || !strings.Contains(out, "exceeds limit") {
		t.Errorf("expected violation details in output, got:\n%s", out)
	}
}

func TestCheckGitHubPR(t *testing.T) {
	res := budget.CheckResult{
		Passed: true,
	}
	var buf bytes.Buffer
	err := report.CheckGitHubPR(&buf, res)
	if err != nil {
		t.Fatalf("CheckGitHubPR returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "<!-- bundlecheck-comment -->") {
		t.Errorf("expected sticky comment marker")
	}
	if !strings.Contains(out, "✅ Angular Bundle Budget Check: PASSED") {
		t.Errorf("expected passed status, got:\n%s", out)
	}
}

func TestComparisonGitHubPRWithFindings(t *testing.T) {
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
	err := report.ComparisonGitHubPR(&buf, comp, budget.CheckResult{Passed: true}, report.TextOptions{Top: 10})
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
			t.Errorf("missing %q in PR output:\n%s", want, out)
		}
	}
}

func TestRenderPRComment_SourceFinding(t *testing.T) {
	res := &comparison.Result{
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
	err := report.ComparisonGitHubPR(&buf, res, budget.CheckResult{Passed: true}, report.TextOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "movie-detail-page") {
		t.Errorf("expected movie-detail-page in PR comment, got:\n%s", out)
	}
	if !strings.Contains(out, "📁") && !strings.Contains(out, "source") {
		t.Errorf("expected source indicator in PR comment, got:\n%s", out)
	}
	wantLine := "- 📁 **`projects/movies/src/app/pages/movie-detail-page`** (`+20 KB`) → emitted in `main.js` *(Application component moved into initial bundle)*"
	if !strings.Contains(out, wantLine) {
		t.Errorf("expected %q in PR comment, got:\n%s", wantLine, out)
	}
}

func TestGitHubPR_ImportPath(t *testing.T) {
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
	err := report.ComparisonGitHubPR(&buf, comp, budget.CheckResult{Passed: true}, report.TextOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	want := "- **Import path:** `apps/portal/src/main.ts` → `apps/portal/src/app/app.module.ts` → `libs/timezone-scheduler/src/index.ts` → `node_modules/moment-timezone/index.js`"
	if !strings.Contains(out, want) {
		t.Errorf("expected formatted import path:\n  want: %s\n  got:\n%s", want, out)
	}
	if strings.Contains(out, "app.config.ts") {
		t.Errorf("expected redundant intermediate hop 'app.config.ts' to be omitted, got:\n%s", out)
	}
}

func TestFormatTracePath(t *testing.T) {
	tests := []struct {
		name string
		path []string
		want string
	}{
		{
			name: "empty path",
			path: nil,
			want: "",
		},
		{
			name: "single hop",
			path: []string{"apps/portal/src/main.ts"},
			want: "`apps/portal/src/main.ts`",
		},
		{
			name: "two hops",
			path: []string{"src/main.ts", "node_modules/chart.js/auto.js"},
			want: "`src/main.ts` → `node_modules/chart.js/auto.js`",
		},
		{
			name: "three hops",
			path: []string{"src/main.ts", "src/app/app.module.ts", "node_modules/chart.js/auto.js"},
			want: "`src/main.ts` → `src/app/app.module.ts` → `node_modules/chart.js/auto.js`",
		},
		{
			name: "four hops preserved",
			path: []string{"apps/portal/src/main.ts", "apps/portal/src/app/app.module.ts", "libs/tz/src/index.ts", "node_modules/tz/index.js"},
			want: "`apps/portal/src/main.ts` → `apps/portal/src/app/app.module.ts` → `libs/tz/src/index.ts` → `node_modules/tz/index.js`",
		},
		{
			name: "five hops with library boundary",
			path: []string{
				"apps/portal/src/main.ts",
				"apps/portal/src/app/app.config.ts",
				"apps/portal/src/app/app.module.ts",
				"libs/timezone-scheduler/src/index.ts",
				"node_modules/moment-timezone/index.js",
			},
			want: "`apps/portal/src/main.ts` → `apps/portal/src/app/app.module.ts` → `libs/timezone-scheduler/src/index.ts` → `node_modules/moment-timezone/index.js`",
		},
		{
			name: "multiple lib hops",
			path: []string{
				"apps/portal/src/main.ts",
				"apps/portal/src/app/app.module.ts",
				"libs/tz/src/index.ts",
				"libs/tz/src/internal.ts",
				"node_modules/tz/index.js",
			},
			want: "`apps/portal/src/main.ts` → `apps/portal/src/app/app.module.ts` → `libs/tz/src/index.ts` → `node_modules/tz/index.js`",
		},
		{
			name: "five hops without library",
			path: []string{
				"src/main.ts",
				"src/app/core.ts",
				"src/app/app.module.ts",
				"src/app/feature.ts",
				"node_modules/chart.js/auto.js",
			},
			want: "`src/main.ts` → `src/app/app.module.ts` → `src/app/feature.ts` → `node_modules/chart.js/auto.js`",
		},
		{
			name: "five internal hops",
			path: []string{
				"src/main.ts",
				"src/app/core.ts",
				"src/app/app.module.ts",
				"src/app/pages/about.ts",
				"src/app/pages/about-details.ts",
			},
			want: "`src/main.ts` → `src/app/app.module.ts` → `src/app/pages/about.ts` → `src/app/pages/about-details.ts`",
		},
		{
			name: "first hop directly imports library",
			path: []string{
				"apps/portal/src/main.ts",
				"libs/tz/src/index.ts",
				"libs/tz/src/internal.ts",
				"libs/tz/src/deep.ts",
				"node_modules/tz/index.js",
			},
			want: "`apps/portal/src/main.ts` → `libs/tz/src/index.ts` → `libs/tz/src/deep.ts` → `node_modules/tz/index.js`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := report.FormatTracePath(tt.path)
			if got != tt.want {
				t.Errorf("FormatTracePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

