package graph

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/snapshot"
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

// TracePackage finds the import chains leading to a given package or module.
func TracePackage(s *snapshot.BundleSnapshot, target string, initialOnly bool, maxChains int) (*WhyResult, error) {
	if s == nil {
		return nil, fmt.Errorf("cannot trace in nil snapshot")
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

	// 1. Build input module adjacency list
	moduleMap := make(map[string]snapshot.Module)
	moduleEdges := make(map[string][]string) // from -> []to

	for _, m := range s.Inputs {
		p := snapshot.CleanPath(m.Path)
		moduleMap[p] = m
	}

	for _, m := range s.Inputs {
		from := snapshot.CleanPath(m.Path)
		for _, imp := range m.Imports {
			if imp.External {
				continue
			}
			to := snapshot.CleanPath(imp.Path)
			if _, exists := moduleMap[to]; !exists {
				// Try relative resolution
				rel := snapshot.CleanPath(path.Join(path.Dir(from), to))
				if _, exists := moduleMap[rel]; exists {
					to = rel
				}
			}
			moduleEdges[from] = append(moduleEdges[from], to)
		}
	}

	// 2. Identify target input modules
	targetInputs := make(map[string]bool)
	var matchedPkgName string

	// Pass 1: exact package match
	for _, m := range s.Inputs {
		p := snapshot.CleanPath(m.Path)
		pkg, isPkg := analysis.PackageName(p)
		if isPkg && strings.EqualFold(pkg, target) {
			targetInputs[p] = true
			matchedPkgName = pkg
		}
	}
	for _, o := range s.Outputs {
		for _, c := range o.Inputs {
			p := snapshot.CleanPath(c.Input)
			pkg, isPkg := analysis.PackageName(p)
			if isPkg && strings.EqualFold(pkg, target) {
				targetInputs[p] = true
				if matchedPkgName == "" {
					matchedPkgName = pkg
				}
			}
		}
	}

	// Pass 2: if no package matched, match file paths without pulling in other third-party packages
	if len(targetInputs) == 0 {
		targetLower := strings.ToLower(target)
		isDependencyPath := strings.Contains(snapshot.CleanPath(target), "node_modules/")
		for _, m := range s.Inputs {
			p := snapshot.CleanPath(m.Path)
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
		for _, o := range s.Outputs {
			for _, c := range o.Inputs {
				p := snapshot.CleanPath(c.Input)
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

	// 3. Compute package bytes in initial / lazy outputs
	targetInOutputs := make(map[string][]struct {
		outputPath string
		entryPoint string
		initial    bool
		bytes      int64
	})

	for _, o := range s.Outputs {
		oPath := snapshot.CleanPath(o.Path)
		for _, c := range o.Inputs {
			inputPath := snapshot.CleanPath(c.Input)
			if targetInputs[inputPath] {
				if o.Initial {
					result.InitialBytes += c.Bytes
				} else {
					result.LazyBytes += c.Bytes
				}
				result.TotalBytes += c.Bytes
				targetInOutputs[inputPath] = append(targetInOutputs[inputPath], struct {
					outputPath string
					entryPoint string
					initial    bool
					bytes      int64
				}{
					outputPath: oPath,
					entryPoint: o.EntryPoint,
					initial:    o.Initial,
					bytes:      c.Bytes,
				})
			}
		}
	}

	if !result.Found {
		return result, nil
	}

	// 4. Determine root entrypoint modules
	var roots []string
	for _, o := range s.Outputs {
		if o.EntryPoint != "" {
			ep := snapshot.CleanPath(o.EntryPoint)
			roots = append(roots, ep)
		} else {
			for _, c := range o.Inputs {
				cPath := snapshot.CleanPath(c.Input)
				if strings.Contains(cPath, "main.") || strings.Contains(cPath, "polyfills.") || strings.Contains(cPath, "index.") {
					roots = append(roots, cPath)
				}
			}
		}
	}

	if len(roots) == 0 {
		for _, m := range s.Inputs {
			p := snapshot.CleanPath(m.Path)
			if !strings.Contains(p, "node_modules") {
				roots = append(roots, p)
			}
		}
	}

	slices.Sort(roots)
	roots = slices.Compact(roots)

	// 5. Build BFS shortest path map
	type bfsQueueItem struct {
		curr string
		path []string
	}

	findModulePath := func(targetInput string) []string {
		queue := []bfsQueueItem{}
		visited := make(map[string]bool)

		for _, r := range roots {
			queue = append(queue, bfsQueueItem{curr: r, path: []string{r}})
			visited[r] = true
		}

		for len(queue) > 0 {
			item := queue[0]
			queue = queue[1:]

			if item.curr == targetInput {
				return item.path
			}

			for _, neighbor := range moduleEdges[item.curr] {
				if !visited[neighbor] {
					visited[neighbor] = true
					newPath := make([]string, len(item.path)+1)
					copy(newPath, item.path)
					newPath[len(item.path)] = neighbor
					queue = append(queue, bfsQueueItem{curr: neighbor, path: newPath})
				}
			}
		}
		return nil
	}

	seenChains := make(map[string]bool)

	// Sort target inputs deterministically
	sortedTargetInputs := make([]string, 0, len(targetInputs))
	for k := range targetInputs {
		sortedTargetInputs = append(sortedTargetInputs, k)
	}
	slices.Sort(sortedTargetInputs)

	for _, targetInput := range sortedTargetInputs {
		modPath := findModulePath(targetInput)

		outputInfos := targetInOutputs[targetInput]
		if len(outputInfos) == 0 {
			outputInfos = append(outputInfos, struct {
				outputPath string
				entryPoint string
				initial    bool
				bytes      int64
			}{
				outputPath: "(unattributed chunk)",
				entryPoint: "",
				initial:    false,
				bytes:      moduleMap[targetInput].Bytes,
			})
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
