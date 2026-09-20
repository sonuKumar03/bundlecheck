package report_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/report"
)

func TestParseBenchmarkOutputAndMarkdown(t *testing.T) {
	raw := `
pkg: bundlecheck/internal/angular
cpu: Apple M4
BenchmarkParse-10    	   83103	     12445 ns/op	    5416 B/op	      46 allocs/op
pkg: bundlecheck/internal/advisor
BenchmarkAdvisorScaling-10    	    1372	    889770 ns/op	 1191148 B/op	    9416 allocs/op
`
	metrics := report.ParseBenchmarkOutput(strings.NewReader(raw))
	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}

	if metrics[0].Name != "BenchmarkParse" || metrics[0].Package != "internal/angular" {
		t.Errorf("unexpected metric 0: %+v", metrics[0])
	}
	if metrics[0].TimePerOp != "12.45 µs" {
		t.Errorf("expected 12.45 µs, got %s", metrics[0].TimePerOp)
	}

	if metrics[1].Name != "BenchmarkAdvisorScaling" || metrics[1].Package != "internal/advisor" {
		t.Errorf("unexpected metric 1: %+v", metrics[1])
	}
	if metrics[1].TimePerOp != "889.77 µs" {
		t.Errorf("expected 889.77 µs, got %s", metrics[1].TimePerOp)
	}

	var buf bytes.Buffer
	err := report.BenchmarkMarkdown(&buf, metrics)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "## ⚡ Benchmark Performance (Human-Readable)") {
		t.Errorf("expected header, got %s", out)
	}
	if !strings.Contains(out, "12.45 µs") || !strings.Contains(out, "889.77 µs") {
		t.Errorf("expected formatted times, got %s", out)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		ns   float64
		want string
	}{
		{500, "500.0 ns"},
		{12445, "12.45 µs"},
		{1500000, "1.50 ms"},
		{2500000000, "2.50 s"},
	}

	for _, tt := range tests {
		got := report.FormatDuration(tt.ns)
		if got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.ns, got, tt.want)
		}
	}
}
