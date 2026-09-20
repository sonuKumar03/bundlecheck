package comparison

import (
	"cmp"
	"slices"
	"strings"

	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/graph"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

// CompareWithSnapshot computes before/after comparison and attaches deterministic regression findings
// explaining what grew, where it landed, and which import introduced it using the current build snapshot.
func CompareWithSnapshot(before, after *analysis.AnalysisResult, snap *snapshot.BundleSnapshot) *Result {
	res := Compare(before, after)
	res.Findings = GenerateFindings(res, snap)
	return res
}

// GenerateFindings builds a ranked list of regression explanations from the comparison result.
// Rules:
// 1. Positive package changes are ranked by bytes, with stable name tie-breakers.
// 2. Changed contributors are associated with emitted chunks when snapshot data is available.
// 3. Shortest stable import paths are traced from entry points via existing graph tracing.
//    No import paths are reconstructed or invented from aggregate baseline JSON.
// 4. Any unexplained byte deltas between the reported total delta and sum of positive packages
//    are explicitly marked as "(unattributed)".
// 5. Findings reconciliation: sum(attributed) + sum(unattributed) == total positive delta.
func GenerateFindings(res *Result, snap *snapshot.BundleSnapshot) []Finding {
	if res == nil {
		return nil
	}

	var findings []Finding
	totalDelta := res.Summary.Delta.TotalJS
	if totalDelta <= 0 && res.Summary.Delta.InitialJS <= 0 {
		return findings
	}

	var g *graph.Graph
	if snap != nil {
		g = graph.NewGraph(snap)
	}

	// Helper to find emitted chunks containing a package
	findEmittedChunks := func(pkgName string) []string {
		if snap == nil {
			return nil
		}
		var chunks []string
		seen := make(map[string]bool)
		pkgLower := strings.ToLower(pkgName)
		for _, out := range snap.Outputs {
			chunkName := snapshot.CleanPath(out.Path)
			for _, in := range out.Inputs {
				p := snapshot.CleanPath(in.Input)
				if pkg, isPkg := analysis.PackageName(p); isPkg && strings.ToLower(pkg) == pkgLower {
					if !seen[chunkName] {
						seen[chunkName] = true
						chunks = append(chunks, chunkName)
					}
					break
				}
			}
		}
		slices.Sort(chunks)
		return chunks
	}

	// Helper to trace import path
	traceImportPath := func(pkgName string) []string {
		if g == nil {
			return nil
		}
		whyRes, err := g.TracePackage(pkgName, false, 1)
		if err == nil && whyRes != nil && len(whyRes.Chains) > 0 {
			return whyRes.Chains[0].Path
		}
		return nil
	}

	var attributedDelta int64

	for _, p := range res.Packages {
		delta := p.Delta.TotalBytes
		if delta <= 0 && p.Delta.InitialBytes <= 0 {
			continue
		}
		effectiveDelta := delta
		if effectiveDelta <= 0 {
			effectiveDelta = p.Delta.InitialBytes
		}

		finding := Finding{
			Name:         p.Name,
			DeltaBytes:   effectiveDelta,
			InitialDelta: p.Delta.InitialBytes,
			LazyDelta:    p.Delta.LazyBytes,
			Kind:         "package",
			Chunks:       findEmittedChunks(p.Name),
			TracePath:    traceImportPath(p.Name),
		}
		attributedDelta += effectiveDelta
		findings = append(findings, finding)
	}

	// Sort package findings by DeltaBytes descending, then by Name ascending
	slices.SortFunc(findings, func(a, b Finding) int {
		if n := cmp.Compare(b.DeltaBytes, a.DeltaBytes); n != 0 {
			return n
		}
		return cmp.Compare(a.Name, b.Name)
	})

	// Check for unattributed delta
	effectiveTotal := totalDelta
	if effectiveTotal <= 0 && res.Summary.Delta.InitialJS > 0 {
		effectiveTotal = res.Summary.Delta.InitialJS
	}

	if effectiveTotal > attributedDelta {
		unattributed := effectiveTotal - attributedDelta
		findings = append(findings, Finding{
			Name:       "(unattributed)",
			DeltaBytes: unattributed,
			Kind:       "unattributed",
			Reason:     "Growth in application sources or chunks not attributed to packages",
		})
	}

	return findings
}
