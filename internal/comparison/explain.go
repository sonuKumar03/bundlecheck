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
//  1. Positive package changes are ranked by bytes, with stable name tie-breakers.
//  2. Changed contributors are associated with emitted chunks when snapshot data is available.
//  3. Shortest stable import paths are traced from entry points via existing graph tracing.
//     No import paths are reconstructed or invented from aggregate baseline JSON.
//  4. Any unexplained byte deltas between the reported total delta and sum of positive packages and sources
//     are explicitly marked as "(unattributed)".
//  5. Findings reconciliation: sum(attributed) + sum(unattributed) == total positive delta.
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

	// Helper to find emitted chunks containing an application source component
	findSourceEmittedChunks := func(srcName string) []string {
		if snap == nil {
			return nil
		}
		var chunks []string
		seen := make(map[string]bool)
		srcLower := strings.ToLower(srcName)
		for _, out := range snap.Outputs {
			chunkName := snapshot.CleanPath(out.Path)
			for _, in := range out.Inputs {
				p := snapshot.CleanPath(in.Input)
				if analysis.IsPackage(p) {
					continue
				}
				dir := strings.ToLower(analysis.SourceDir(p))
				pLower := strings.ToLower(p)
				if dir == srcLower || pLower == srcLower || strings.HasPrefix(pLower, srcLower+"/") {
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
	traceImportPath := func(target string) []string {
		if g == nil {
			return nil
		}
		whyRes, err := g.TracePackage(target, false, 1)
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

	// Check for unattributed delta or source attribution
	effectiveTotal := totalDelta
	if effectiveTotal <= 0 && res.Summary.Delta.InitialJS > 0 {
		effectiveTotal = res.Summary.Delta.InitialJS
	}

	if effectiveTotal > attributedDelta && len(res.Sources) > 0 {
		var candidateSources []Finding
		for _, s := range res.Sources {
			delta := s.Delta.TotalBytes
			if delta <= 0 && s.Delta.InitialBytes <= 0 {
				continue
			}
			effectiveDelta := delta
			if effectiveDelta <= 0 {
				effectiveDelta = s.Delta.InitialBytes
			}

			reason := "Application source code growth"
			if s.Delta.InitialBytes > 0 && s.Before.InitialBytes == 0 && s.Before.LazyBytes > 0 {
				reason = "Application component moved into initial bundle"
			}

			candidateSources = append(candidateSources, Finding{
				Name:         s.Name,
				DeltaBytes:   effectiveDelta,
				InitialDelta: s.Delta.InitialBytes,
				LazyDelta:    s.Delta.LazyBytes,
				Kind:         "source",
				Chunks:       findSourceEmittedChunks(s.Name),
				TracePath:    traceImportPath(s.Name),
				Reason:       reason,
			})
		}

		// Rank positive source changes by byte delta descending, then Name ascending
		slices.SortFunc(candidateSources, func(a, b Finding) int {
			if n := cmp.Compare(b.DeltaBytes, a.DeltaBytes); n != 0 {
				return n
			}
			return cmp.Compare(a.Name, b.Name)
		})

		for _, f := range candidateSources {
			if attributedDelta >= effectiveTotal {
				break
			}
			attributedDelta += f.DeltaBytes
			findings = append(findings, f)
		}
	} else if effectiveTotal > attributedDelta && snap != nil {
		type appSource struct {
			name  string
			bytes int64
		}
		appSourcesMap := make(map[string]int64)
		for _, out := range snap.Outputs {
			if !out.Initial {
				continue
			}
			for _, in := range out.Inputs {
				p := snapshot.CleanPath(in.Input)
				if analysis.IsPackage(p) || in.Bytes == 0 {
					continue
				}
				dir := analysis.SourceDir(p)
				appSourcesMap[dir] += in.Bytes
			}
		}

		var candidateAppSources []appSource
		for name, b := range appSourcesMap {
			candidateAppSources = append(candidateAppSources, appSource{name: name, bytes: b})
		}
		slices.SortFunc(candidateAppSources, func(a, b appSource) int {
			if n := cmp.Compare(b.bytes, a.bytes); n != 0 {
				return n
			}
			return cmp.Compare(a.name, b.name)
		})

		for _, src := range candidateAppSources {
			if attributedDelta >= effectiveTotal {
				break
			}
			remaining := effectiveTotal - attributedDelta
			delta := src.bytes
			if delta > remaining {
				delta = remaining
			}
			f := Finding{
				Name:         src.name,
				DeltaBytes:   delta,
				InitialDelta: delta,
				Kind:         "source",
				Chunks:       findSourceEmittedChunks(src.name),
				TracePath:    traceImportPath(src.name),
				Reason:       "Application source in initial chunk",
			}
			attributedDelta += delta
			findings = append(findings, f)
		}
	}

	// Sort package and source findings by DeltaBytes descending, then by Name ascending
	slices.SortFunc(findings, func(a, b Finding) int {
		if n := cmp.Compare(b.DeltaBytes, a.DeltaBytes); n != 0 {
			return n
		}
		return cmp.Compare(a.Name, b.Name)
	})

	// Check for remaining unattributed delta
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
