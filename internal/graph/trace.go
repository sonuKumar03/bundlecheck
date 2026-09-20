package graph

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

type TraceChain struct {
	Output       string   `json:"output"`
	Initial      bool     `json:"initial"`
	Path         []string `json:"path"`
	BytesInChunk int64    `json:"bytesInChunk"`
}

type WhyResult struct {
	SchemaVersion string       `json:"schemaVersion"`
	ToolVersion   string       `json:"toolVersion"`
	Command       string       `json:"command"`
	Target        string       `json:"target"`
	PackageName   string       `json:"packageName,omitempty"`
	Found         bool         `json:"found"`
	InitialBytes  int64        `json:"initialBytes"`
	LazyBytes     int64        `json:"lazyBytes"`
	TotalBytes    int64        `json:"totalBytes"`
	Chains        []TraceChain `json:"chains"`
}

type targetOutputInfo struct {
	outputPath string
	entryPoint string
	initial    bool
	bytes      int64
}

// Graph holds the pre-indexed module dependency graph and traversal state for fast tracing.
type Graph struct {
	Snapshot       *snapshot.BundleSnapshot
	ModuleMap      map[string]snapshot.Module
	ModuleEdges    map[string][]string
	Roots          []string
	ParentMap      map[string]string // node -> predecessor in shortest path from roots
	Visited        map[string]bool
	PackageInputs  map[string][]string // lowercase pkgName -> slice of input paths
	PackageCanon   map[string]string   // lowercase pkgName -> canonical pkgName
	InputToOutputs map[string][]targetOutputInfo
}

// NewGraph builds an indexed module graph and computes shortest paths from roots in a single BFS pass.
func NewGraph(s *snapshot.BundleSnapshot) *Graph {
	if s == nil {
		return nil
	}

	g := &Graph{
		Snapshot:       s,
		ModuleMap:      make(map[string]snapshot.Module, len(s.Inputs)),
		ModuleEdges:    make(map[string][]string, len(s.Inputs)),
		ParentMap:      make(map[string]string, len(s.Inputs)),
		Visited:        make(map[string]bool, len(s.Inputs)),
		PackageInputs:  make(map[string][]string),
		PackageCanon:   make(map[string]string),
		InputToOutputs: make(map[string][]targetOutputInfo, len(s.Inputs)),
	}

	seenPkgInput := make(map[string]bool)

	for _, m := range s.Inputs {
		p := snapshot.CleanPath(m.Path)
		g.ModuleMap[p] = m
		if pkg, isPkg := analysis.PackageName(p); isPkg {
			pkgLower := strings.ToLower(pkg)
			g.PackageCanon[pkgLower] = pkg
			key := pkgLower + "::" + p
			if !seenPkgInput[key] {
				seenPkgInput[key] = true
				g.PackageInputs[pkgLower] = append(g.PackageInputs[pkgLower], p)
			}
		}
	}

	for _, m := range s.Inputs {
		from := snapshot.CleanPath(m.Path)
		for _, imp := range m.Imports {
			if imp.External {
				continue
			}
			to := snapshot.CleanPath(imp.Path)
			if _, exists := g.ModuleMap[to]; !exists {
				// Try relative resolution
				rel := snapshot.CleanPath(path.Join(path.Dir(from), to))
				if _, exists := g.ModuleMap[rel]; exists {
					to = rel
				}
			}
			g.ModuleEdges[from] = append(g.ModuleEdges[from], to)
		}
	}

	// Index output contributions and root entrypoints
	for _, o := range s.Outputs {
		oPath := snapshot.CleanPath(o.Path)
		if o.EntryPoint != "" {
			ep := snapshot.CleanPath(o.EntryPoint)
			g.Roots = append(g.Roots, ep)
		} else {
			for _, c := range o.Inputs {
				cPath := snapshot.CleanPath(c.Input)
				if strings.Contains(cPath, "main.") || strings.Contains(cPath, "polyfills.") || strings.Contains(cPath, "index.") {
					g.Roots = append(g.Roots, cPath)
				}
			}
		}

		for _, c := range o.Inputs {
			cPath := snapshot.CleanPath(c.Input)
			if pkg, isPkg := analysis.PackageName(cPath); isPkg {
				pkgLower := strings.ToLower(pkg)
				if g.PackageCanon[pkgLower] == "" {
					g.PackageCanon[pkgLower] = pkg
				}
				key := pkgLower + "::" + cPath
				if !seenPkgInput[key] {
					seenPkgInput[key] = true
					g.PackageInputs[pkgLower] = append(g.PackageInputs[pkgLower], cPath)
				}
			}

			g.InputToOutputs[cPath] = append(g.InputToOutputs[cPath], targetOutputInfo{
				outputPath: oPath,
				entryPoint: o.EntryPoint,
				initial:    o.Initial,
				bytes:      c.Bytes,
			})
		}
	}

	if len(g.Roots) == 0 {
		for _, m := range s.Inputs {
			p := snapshot.CleanPath(m.Path)
			if !strings.Contains(p, "node_modules") {
				g.Roots = append(g.Roots, p)
			}
		}
	}

	slices.Sort(g.Roots)
	g.Roots = slices.Compact(g.Roots)

	// Single BFS pass to compute shortest path parent tree from all roots
	queue := make([]string, 0, len(g.Roots)+len(s.Inputs))
	for _, r := range g.Roots {
		g.Visited[r] = true
		g.ParentMap[r] = ""
		queue = append(queue, r)
	}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, neighbor := range g.ModuleEdges[curr] {
			if !g.Visited[neighbor] {
				g.Visited[neighbor] = true
				g.ParentMap[neighbor] = curr
				queue = append(queue, neighbor)
			}
		}
	}

	return g
}

// ShortestPath reconstructs the shortest path from roots to the target input module in O(depth) time.
func (g *Graph) ShortestPath(targetInput string) []string {
	if !g.Visited[targetInput] {
		return nil
	}

	path := []string{targetInput}
	curr := targetInput
	for {
		parent, exists := g.ParentMap[curr]
		if !exists || parent == "" {
			break
		}
		path = append(path, parent)
		curr = parent
	}
	slices.Reverse(path)
	return path
}

// TracePackage traces import chains for a package or module on a pre-indexed graph.
func (g *Graph) TracePackage(target string, initialOnly bool, maxChains int) (*WhyResult, error) {
	if g == nil || g.Snapshot == nil {
		return nil, fmt.Errorf("cannot trace in nil graph")
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("target package or path cannot be empty")
	}
	if maxChains <= 0 {
		maxChains = 5
	}

	result := &WhyResult{
		SchemaVersion: "1",
		ToolVersion:   analysis.ToolVersion,
		Command:       "why",
		Target:        target,
		Chains:        []TraceChain{},
	}

	targetLower := strings.ToLower(target)
	targetInputs := make(map[string]bool)
	var matchedPkgName string

	// Pass 1: exact package match via pre-indexed map O(1)
	if inputs, isPkg := g.PackageInputs[targetLower]; isPkg && len(inputs) > 0 {
		for _, inp := range inputs {
			targetInputs[inp] = true
		}
		matchedPkgName = g.PackageCanon[targetLower]
	}

	// Pass 2: if no package matched, match file paths without pulling in other third-party packages
	if len(targetInputs) == 0 {
		isDependencyPath := strings.Contains(snapshot.CleanPath(target), "node_modules/")
		for _, module := range g.Snapshot.Inputs {
			p := snapshot.CleanPath(module.Path)
			pLower := strings.ToLower(p)
			if pLower == targetLower || strings.Contains(pLower, targetLower) {
				pkg, isPkg := analysis.PackageName(p)
				if !isPkg || pLower == targetLower || isDependencyPath {
					targetInputs[p] = true
					if isPkg && matchedPkgName == "" {
						matchedPkgName = pkg
					}
				}
			}
		}
		for _, output := range g.Snapshot.Outputs {
			for _, contribution := range output.Inputs {
				p := snapshot.CleanPath(contribution.Input)
				pLower := strings.ToLower(p)
				if pLower == targetLower || strings.Contains(pLower, targetLower) {
					pkg, isPkg := analysis.PackageName(p)
					if !isPkg || pLower == targetLower || isDependencyPath {
						targetInputs[p] = true
						if isPkg && matchedPkgName == "" {
							matchedPkgName = pkg
						}
					}
				}
			}
		}
	}

	result.PackageName = matchedPkgName
	result.Found = len(targetInputs) > 0

	if !result.Found {
		return result, nil
	}

	// 2. Compute package bytes in initial / lazy outputs from pre-indexed outputs
	for targetInput := range targetInputs {
		for _, info := range g.InputToOutputs[targetInput] {
			if info.initial {
				result.InitialBytes += info.bytes
			} else {
				result.LazyBytes += info.bytes
			}
			result.TotalBytes += info.bytes
		}
	}

	seenChains := make(map[string]bool)

	// Sort target inputs deterministically
	sortedTargetInputs := make([]string, 0, len(targetInputs))
	for k := range targetInputs {
		sortedTargetInputs = append(sortedTargetInputs, k)
	}
	slices.Sort(sortedTargetInputs)

	for _, targetInput := range sortedTargetInputs {
		modPath := g.ShortestPath(targetInput)

		outputInfos := g.InputToOutputs[targetInput]
		if len(outputInfos) == 0 {
			outputInfos = []targetOutputInfo{{
				outputPath: "(unattributed chunk)",
				entryPoint: "",
				initial:    false,
				bytes:      g.ModuleMap[targetInput].Bytes,
			}}
		}

		for _, info := range outputInfos {
			if initialOnly && !info.initial {
				continue
			}

			var chainPath []string
			if len(modPath) > 0 {
				chainPath = modPath
			} else if info.entryPoint != "" && info.entryPoint != targetInput {
				chainPath = []string{info.entryPoint, targetInput}
			} else {
				chainPath = []string{targetInput}
			}

			chainKey := info.outputPath + "::" + strings.Join(chainPath, "->")
			if !seenChains[chainKey] {
				seenChains[chainKey] = true
				result.Chains = append(result.Chains, TraceChain{
					Output:       info.outputPath,
					Initial:      info.initial,
					Path:         chainPath,
					BytesInChunk: info.bytes,
				})
				if len(result.Chains) >= maxChains {
					return result, nil
				}
			}
		}
	}

	return result, nil
}

// TracePackage finds the import chains leading to a given package or module.
func TracePackage(s *snapshot.BundleSnapshot, target string, initialOnly bool, maxChains int) (*WhyResult, error) {
	if s == nil {
		return nil, fmt.Errorf("cannot trace in nil snapshot")
	}
	g := NewGraph(s)
	return g.TracePackage(target, initialOnly, maxChains)
}
