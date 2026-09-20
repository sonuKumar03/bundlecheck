// Package comparison computes changes between validated summary documents.
package comparison

import (
	"cmp"
	"slices"

	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

type Bytes struct {
	InitialBytes int64 `json:"initialBytes"`
	LazyBytes    int64 `json:"lazyBytes"`
	TotalBytes   int64 `json:"totalBytes"`
}

type SummaryChange struct {
	Before snapshot.Totals `json:"before"`
	After  snapshot.Totals `json:"after"`
	Delta  snapshot.Totals `json:"delta"`
}

type PackageChange struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Before Bytes  `json:"before"`
	After  Bytes  `json:"after"`
	Delta  Bytes  `json:"delta"`
}

type Result struct {
	SchemaVersion string          `json:"schemaVersion"`
	ToolVersion   string          `json:"toolVersion"`
	Command       string          `json:"command"`
	Summary       SummaryChange   `json:"summary"`
	Packages      []PackageChange `json:"packages"`
}

// Compare consumes summaries validated by Parse. Nonnegative int64 operands
// ensure subtraction and absolute deltas cannot overflow or reach MinInt64.
func Compare(before, after *analysis.AnalysisResult) *Result {
	r := &Result{SchemaVersion: "1", ToolVersion: analysis.ToolVersion, Command: "compare", Packages: []PackageChange{}}
	r.Summary = SummaryChange{Before: before.Summary, After: after.Summary, Delta: snapshot.Totals{
		InitialJS: after.Summary.InitialJS - before.Summary.InitialJS,
		LazyJS:    after.Summary.LazyJS - before.Summary.LazyJS,
		TotalJS:   after.Summary.TotalJS - before.Summary.TotalJS,
	}}
	old := make(map[string]snapshot.Package)
	new := make(map[string]snapshot.Package)
	for _, p := range before.Packages {
		old[p.Name] = p
	}
	for _, p := range after.Packages {
		new[p.Name] = p
	}
	appendPackage := func(name string) {
		b, existed := old[name]
		a, exists := new[name]
		p := PackageChange{Name: name, Status: "changed",
			Before: Bytes{b.InitialBytes, b.LazyBytes, b.TotalBytes},
			After:  Bytes{a.InitialBytes, a.LazyBytes, a.TotalBytes},
			Delta:  Bytes{a.InitialBytes - b.InitialBytes, a.LazyBytes - b.LazyBytes, a.TotalBytes - b.TotalBytes},
		}
		switch {
		case !existed:
			p.Status = "added"
		case !exists:
			p.Status = "removed"
		case p.Delta == (Bytes{}):
			p.Status = "unchanged"
		}
		r.Packages = append(r.Packages, p)
	}
	for name := range old {
		appendPackage(name)
	}
	for name := range new {
		if _, exists := old[name]; !exists {
			appendPackage(name)
		}
	}
	slices.SortFunc(r.Packages, func(a, b PackageChange) int {
		if n := cmp.Compare(abs(b.Delta.InitialBytes), abs(a.Delta.InitialBytes)); n != 0 {
			return n
		}
		if n := cmp.Compare(abs(b.Delta.LazyBytes), abs(a.Delta.LazyBytes)); n != 0 {
			return n
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return r
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
