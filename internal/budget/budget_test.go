package budget_test

import (
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/budget"
	"github.com/sonuKumar03/bundlecheck/internal/comparison"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

func TestParseBytes(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		err      bool
	}{
		{"1024", 1024, false},
		{"1024B", 1024, false},
		{"1KB", 1024, false},
		{"1.5KB", 1536, false},
		{"200KB", 204800, false},
		{"1MB", 1048576, false},
		{"1.5MB", 1572864, false},
		{"-10KB", -10240, false},
		{"0", 0, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tc := range tests {
		got, err := budget.ParseBytes(tc.input)
		if (err != nil) != tc.err {
			t.Errorf("ParseBytes(%q) error = %v, expected err %v", tc.input, err, tc.err)
			continue
		}
		if !tc.err && got != tc.expected {
			t.Errorf("ParseBytes(%q) = %d, expected %d", tc.input, got, tc.expected)
		}
	}
}

func TestCheckSummary(t *testing.T) {
	totals := snapshot.Totals{
		InitialJS: 200 * 1024,
		LazyJS:    300 * 1024,
		TotalJS:   500 * 1024,
	}

	maxInitial := int64(250 * 1024)
	resPass := budget.CheckSummary(totals, budget.Limits{
		MaxInitial: &maxInitial,
	})
	if !resPass.Passed {
		t.Errorf("expected pass, got fail with violations: %+v", resPass.Violations)
	}

	maxInitialFail := int64(150 * 1024)
	resFail := budget.CheckSummary(totals, budget.Limits{
		MaxInitial: &maxInitialFail,
	})
	if resFail.Passed || len(resFail.Violations) != 1 {
		t.Errorf("expected 1 violation, got passed=%v, count=%d", resFail.Passed, len(resFail.Violations))
	}
}

func TestCheckComparison(t *testing.T) {
	comp := &comparison.Result{
		Summary: comparison.SummaryChange{
			Before: snapshot.Totals{InitialJS: 1000, TotalJS: 2000},
			After:  snapshot.Totals{InitialJS: 1200, TotalJS: 2100},
			Delta:  snapshot.Totals{InitialJS: 200, TotalJS: 100},
		},
	}

	allowedDelta := int64(300)
	resPass := budget.CheckComparison(comp, budget.Limits{
		MaxInitialDelta: &allowedDelta,
	})
	if !resPass.Passed {
		t.Errorf("expected pass, got violations: %+v", resPass.Violations)
	}

	allowedDeltaFail := int64(100)
	resFail := budget.CheckComparison(comp, budget.Limits{
		MaxInitialDelta: &allowedDeltaFail,
	})
	if resFail.Passed || len(resFail.Violations) != 1 {
		t.Errorf("expected fail on initial delta, got passed=%v", resFail.Passed)
	}
}
