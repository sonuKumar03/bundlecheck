package report

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
)

type BenchmarkMetric struct {
	Name       string
	Package    string
	Iterations int64
	TimePerOp  string
	RawNs      float64
	BytesPerOp string
	Allocs     string
}

func FormatDuration(ns float64) string {
	if ns < 1000 {
		return fmt.Sprintf("%.1f ns", ns)
	} else if ns < 1_000_000 {
		return fmt.Sprintf("%.2f µs", ns/1000.0)
	} else if ns < 1_000_000_000 {
		return fmt.Sprintf("%.2f ms", ns/1_000_000.0)
	}
	return fmt.Sprintf("%.2f s", ns/1_000_000_000.0)
}

// ParseBenchmarkOutput parses standard `go test -bench=. -benchmem` output.
func ParseBenchmarkOutput(r io.Reader) []BenchmarkMetric {
	scanner := bufio.NewScanner(r)
	benchRegex := regexp.MustCompile(`^(Benchmark\w+)(?:-\d+)?\s+(\d+)\s+([\d.]+)\s+ns/op(?:\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op)?`)

	var metrics []BenchmarkMetric
	currentPkg := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "pkg: ") {
			currentPkg = strings.TrimPrefix(line, "pkg: bundleradar/")
			continue
		}

		match := benchRegex.FindStringSubmatch(line)
		if len(match) > 3 {
			name := match[1]
			iters, _ := strconv.ParseInt(match[2], 10, 64)
			ns, _ := strconv.ParseFloat(match[3], 64)

			bOpStr := "-"
			allocsStr := "-"

			if len(match) > 5 && match[4] != "" {
				bVal, _ := strconv.ParseInt(match[4], 10, 64)
				bOpStr = formatBytes(bVal)
			}
			if len(match) > 5 && match[5] != "" {
				allocVal, _ := strconv.ParseInt(match[5], 10, 64)
				allocsStr = fmt.Sprintf("%d", allocVal)
			}

			metrics = append(metrics, BenchmarkMetric{
				Name:       name,
				Package:    currentPkg,
				Iterations: iters,
				TimePerOp:  FormatDuration(ns),
				RawNs:      ns,
				BytesPerOp: bOpStr,
				Allocs:     allocsStr,
			})
		}
	}

	return metrics
}

// BenchmarkMarkdown renders parsed benchmark metrics as a readable GitHub Flavored Markdown table.
func BenchmarkMarkdown(w io.Writer, metrics []BenchmarkMetric) error {
	if len(metrics) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("## ⚡ Benchmark Performance (Human-Readable)\n\n")
	sb.WriteString("| Benchmark Suite | Package | Time / Op | Memory / Op | Allocations |\n")
	sb.WriteString("| :--- | :--- | :---: | :---: | :---: |\n")

	for _, m := range metrics {
		pkg := "-"
		if m.Package != "" {
			pkg = fmt.Sprintf("`%s`", m.Package)
		}
		fmt.Fprintf(&sb, "| **`%s`** | %s | `%s` | `%s` | `%s` |\n",
			m.Name, pkg, m.TimePerOp, m.BytesPerOp, m.Allocs)
	}
	sb.WriteString("\n")

	_, err := io.WriteString(w, sb.String())
	return err
}

// BenchmarkText renders parsed benchmark metrics as a readable CLI table.
func BenchmarkText(w io.Writer, metrics []BenchmarkMetric) error {
	if len(metrics) == 0 {
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Benchmark Performance (Human-Readable)")
	fmt.Fprintln(tw, "Benchmark Suite\tPackage\tTime / Op\tMemory / Op\tAllocations")

	for _, m := range metrics {
		pkg := m.Package
		if pkg == "" {
			pkg = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			m.Name, pkg, m.TimePerOp, m.BytesPerOp, m.Allocs)
	}

	return tw.Flush()
}
