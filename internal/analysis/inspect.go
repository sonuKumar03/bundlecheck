package analysis

import (
	"cmp"
	"fmt"
	"path"
	"slices"

	"github.com/sonuKumar03/bundleradar/internal/snapshot"
)

type Contributor struct {
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}

type InspectResult struct {
	SchemaVersion string        `json:"schemaVersion"`
	ToolVersion   string        `json:"toolVersion"`
	Command       string        `json:"command"`
	Chunk         string        `json:"chunk"`
	Bytes         int64         `json:"bytes"`
	Initial       bool          `json:"initial"`
	EntryPoint    string        `json:"entryPoint,omitempty"`
	Packages      []Contributor `json:"packages"`
	Modules       []Contributor `json:"modules"`
}

func InspectChunk(s *snapshot.BundleSnapshot, target string) (*InspectResult, error) {
	if s == nil {
		return nil, fmt.Errorf("cannot inspect nil snapshot")
	}
	target = snapshot.CleanPath(target)
	var matches []snapshot.BundleOutput
	for _, output := range s.Outputs {
		if snapshot.CleanPath(output.Path) == target {
			matches = []snapshot.BundleOutput{output}
			break
		}
		if path.Base(target) == target && path.Base(snapshot.CleanPath(output.Path)) == target {
			matches = append(matches, output)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("chunk %q not found", target)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("chunk filename %q is ambiguous; use its full output path", target)
	}
	output := matches[0]
	result := &InspectResult{
		SchemaVersion: s.SchemaVersion,
		ToolVersion:   ToolVersion,
		Command:       "inspect",
		Chunk:         snapshot.CleanPath(output.Path),
		Bytes:         output.Bytes,
		Initial:       output.Initial,
		EntryPoint:    output.EntryPoint,
		Packages:      []Contributor{},
		Modules:       make([]Contributor, 0, len(output.Inputs)),
	}
	packages := map[string]int64{}
	for _, contribution := range output.Inputs {
		name := snapshot.CleanPath(contribution.Input)
		result.Modules = append(result.Modules, Contributor{Name: name, Bytes: contribution.Bytes})
		if pkg, ok := PackageName(name); ok {
			n := packages[pkg]
			if err := add(&n, contribution.Bytes); err != nil {
				return nil, fmt.Errorf("package %q: %w", pkg, err)
			}
			packages[pkg] = n
		}
	}
	for name, bytes := range packages {
		result.Packages = append(result.Packages, Contributor{Name: name, Bytes: bytes})
	}
	sortContributors(result.Packages)
	sortContributors(result.Modules)
	return result, nil
}

func sortContributors(items []Contributor) {
	slices.SortFunc(items, func(a, b Contributor) int {
		if n := cmp.Compare(b.Bytes, a.Bytes); n != 0 {
			return n
		}
		return cmp.Compare(a.Name, b.Name)
	})
}
