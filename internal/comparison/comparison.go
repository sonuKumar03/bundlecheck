// Package comparison computes changes between validated summary documents.
package comparison

import (
	"cmp"
	"slices"

	"github.com/sonuKumar03/bundleradar/internal/analysis"
	"github.com/sonuKumar03/bundleradar/internal/snapshot"
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

type Finding struct {
	Name         string   `json:"name"`
	DeltaBytes   int64    `json:"deltaBytes"`
	InitialDelta int64    `json:"initialDelta,omitempty"`
	LazyDelta    int64    `json:"lazyDelta,omitempty"`
	Kind         string   `json:"kind"` // "package", "source", or "unattributed"
	Chunks       []string `json:"chunks,omitempty"`
	TracePath    []string `json:"tracePath,omitempty"`
	Reason       string   `json:"reason,omitempty"`
}

type SourceChange struct {
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
	Sources       []SourceChange  `json:"sources,omitempty"`
	Findings      []Finding       `json:"findings,omitempty"`
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
	if before.Sources != nil {
		r.Sources = []SourceChange{}
		oldSrc := make(map[string]analysis.SourceContribution)
		newSrc := make(map[string]analysis.SourceContribution)
		for _, s := range before.Sources {
			oldSrc[s.Name] = s
		}
		for _, s := range after.Sources {
			newSrc[s.Name] = s
		}
		appendSource := func(name string) {
			b, existed := oldSrc[name]
			a, exists := newSrc[name]
			s := SourceChange{Name: name, Status: "changed",
				Before: Bytes{b.InitialBytes, b.LazyBytes, b.TotalBytes},
				After:  Bytes{a.InitialBytes, a.LazyBytes, a.TotalBytes},
				Delta:  Bytes{a.InitialBytes - b.InitialBytes, a.LazyBytes - b.LazyBytes, a.TotalBytes - b.TotalBytes},
			}
			switch {
			case !existed:
				s.Status = "added"
			case !exists:
				s.Status = "removed"
			case s.Delta == (Bytes{}):
				s.Status = "unchanged"
			}
			r.Sources = append(r.Sources, s)
		}
		for name := range oldSrc {
			appendSource(name)
		}
		for name := range newSrc {
			if _, exists := oldSrc[name]; !exists {
				appendSource(name)
			}
		}
		slices.SortFunc(r.Sources, func(a, b SourceChange) int {
			if n := cmp.Compare(abs(b.Delta.InitialBytes), abs(a.Delta.InitialBytes)); n != 0 {
				return n
			}
			if n := cmp.Compare(abs(b.Delta.LazyBytes), abs(a.Delta.LazyBytes)); n != 0 {
				return n
			}
			return cmp.Compare(a.Name, b.Name)
		})
	}
	return r
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
