// Package budget validates bundle sizes and regressions against configurable thresholds.
package budget

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sonuKumar03/bundlecheck/internal/comparison"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

type Limits struct {
	MaxInitial      *int64 `json:"maxInitial,omitempty"`
	MaxLazy         *int64 `json:"maxLazy,omitempty"`
	MaxTotal        *int64 `json:"maxTotal,omitempty"`
	MaxInitialDelta *int64 `json:"maxInitialDelta,omitempty"`
	MaxTotalDelta   *int64 `json:"maxTotalDelta,omitempty"`
}

type Violation struct {
	Metric  string `json:"metric"`
	Actual  int64  `json:"actual"`
	Limit   int64  `json:"limit"`
	Message string `json:"message"`
}

type CheckResult struct {
	Passed     bool               `json:"passed"`
	Violations []Violation        `json:"violations"`
	Summary    *snapshot.Totals   `json:"summary,omitempty"`
	Comparison *comparison.Result `json:"comparison,omitempty"`
}

// ParseBytes converts size strings like "200KB", "1.5MB", "1024B", "-10KB" into signed int64 bytes.
func ParseBytes(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty byte string")
	}

	negative := false
	if strings.HasPrefix(s, "-") {
		negative = true
		s = strings.TrimSpace(strings.TrimPrefix(s, "-"))
	} else if strings.HasPrefix(s, "+") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "+"))
	}

	upper := strings.ToUpper(s)
	multiplier := int64(1)

	switch {
	case strings.HasSuffix(upper, "KB") || strings.HasSuffix(upper, "K"):
		multiplier = 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "KB"), "kb")
		s = strings.TrimSuffix(strings.TrimSuffix(s, "Kb"), "k")
		s = strings.TrimSuffix(s, "K")
	case strings.HasSuffix(upper, "MB") || strings.HasSuffix(upper, "M"):
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "MB"), "mb")
		s = strings.TrimSuffix(strings.TrimSuffix(s, "Mb"), "m")
		s = strings.TrimSuffix(s, "M")
	case strings.HasSuffix(upper, "GB") || strings.HasSuffix(upper, "G"):
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "GB"), "gb")
		s = strings.TrimSuffix(s, "G")
	case strings.HasSuffix(upper, "B"):
		multiplier = 1
		s = strings.TrimSuffix(strings.TrimSuffix(s, "B"), "b")
	}

	s = strings.TrimSpace(s)
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q: %w", s, err)
	}

	bytesFloat := val * float64(multiplier)
	if bytesFloat > float64(math.MaxInt64) {
		return 0, fmt.Errorf("byte value exceeds int64 max")
	}

	res := int64(bytesFloat)
	if negative {
		res = -res
	}
	return res, nil
}

// CheckSummary verifies absolute bundle metrics against limits.
func CheckSummary(totals snapshot.Totals, limits Limits) CheckResult {
	res := CheckResult{Passed: true, Violations: []Violation{}}

	if limits.MaxInitial != nil && totals.InitialJS > *limits.MaxInitial {
		res.Passed = false
		res.Violations = append(res.Violations, Violation{
			Metric:  "initialJs",
			Actual:  totals.InitialJS,
			Limit:   *limits.MaxInitial,
			Message: fmt.Sprintf("initial JS size (%d bytes) exceeds budget of %d bytes", totals.InitialJS, *limits.MaxInitial),
		})
	}

	if limits.MaxLazy != nil && totals.LazyJS > *limits.MaxLazy {
		res.Passed = false
		res.Violations = append(res.Violations, Violation{
			Metric:  "lazyJs",
			Actual:  totals.LazyJS,
			Limit:   *limits.MaxLazy,
			Message: fmt.Sprintf("lazy JS size (%d bytes) exceeds budget of %d bytes", totals.LazyJS, *limits.MaxLazy),
		})
	}

	if limits.MaxTotal != nil && totals.TotalJS > *limits.MaxTotal {
		res.Passed = false
		res.Violations = append(res.Violations, Violation{
			Metric:  "totalJs",
			Actual:  totals.TotalJS,
			Limit:   *limits.MaxTotal,
			Message: fmt.Sprintf("total JS size (%d bytes) exceeds budget of %d bytes", totals.TotalJS, *limits.MaxTotal),
		})
	}

	return res
}

// CheckComparison verifies comparison deltas against regression limits.
func CheckComparison(comp *comparison.Result, limits Limits) CheckResult {
	res := CheckResult{Passed: true, Violations: []Violation{}}
	if comp == nil {
		return res
	}

	// Check summary absolute limits against After
	summaryRes := CheckSummary(comp.Summary.After, limits)
	if !summaryRes.Passed {
		res.Passed = false
		res.Violations = append(res.Violations, summaryRes.Violations...)
	}

	// Check deltas
	if limits.MaxInitialDelta != nil && comp.Summary.Delta.InitialJS > *limits.MaxInitialDelta {
		res.Passed = false
		res.Violations = append(res.Violations, Violation{
			Metric:  "initialJsDelta",
			Actual:  comp.Summary.Delta.InitialJS,
			Limit:   *limits.MaxInitialDelta,
			Message: fmt.Sprintf("initial JS increase (%+d bytes) exceeds allowed limit of %+d bytes", comp.Summary.Delta.InitialJS, *limits.MaxInitialDelta),
		})
	}

	if limits.MaxTotalDelta != nil && comp.Summary.Delta.TotalJS > *limits.MaxTotalDelta {
		res.Passed = false
		res.Violations = append(res.Violations, Violation{
			Metric:  "totalJsDelta",
			Actual:  comp.Summary.Delta.TotalJS,
			Limit:   *limits.MaxTotalDelta,
			Message: fmt.Sprintf("total JS increase (%+d bytes) exceeds allowed limit of %+d bytes", comp.Summary.Delta.TotalJS, *limits.MaxTotalDelta),
		})
	}

	return res
}
