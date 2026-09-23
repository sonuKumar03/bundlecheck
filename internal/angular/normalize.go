package angular

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/sonuKumar03/bundleradar/internal/snapshot"
)

func Normalize(m *Metafile) (*snapshot.BundleSnapshot, error) {
	if m == nil {
		return nil, fmt.Errorf("cannot normalize nil stats")
	}
	s := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Inputs:        []snapshot.Module{},
		Outputs:       []snapshot.BundleOutput{},
		Packages:      []snapshot.Package{},
	}

	modules := make(map[string]snapshot.Module)
	for _, p := range keys(m.Inputs) {
		name := snapshot.CleanPath(p)
		raw := m.Inputs[p]
		if existing, exists := modules[name]; exists && existing.Bytes != raw.Bytes {
			return nil, fmt.Errorf("conflicting normalized input %q", name)
		}

		mod := snapshot.Module{
			Path:    name,
			Bytes:   raw.Bytes,
			Imports: []snapshot.Import{},
		}
		for _, imp := range raw.Imports {
			impPath := imp.Path
			if !imp.External {
				impPath = snapshot.CleanPath(impPath)
			}
			mod.Imports = append(mod.Imports, snapshot.Import{
				Path:     impPath,
				Dynamic:  imp.Kind == "dynamic-import",
				External: imp.External,
				Asset:    imp.Kind == "file-loader",
			})
		}
		modules[name] = mod
	}
	for _, p := range keys(modules) {
		s.Inputs = append(s.Inputs, modules[p])
	}

	seen := make(map[string]bool)
	for _, p := range keys(m.Outputs) {
		raw := m.Outputs[p]
		name := snapshot.CleanPath(p)
		if seen[name] {
			return nil, fmt.Errorf("duplicate normalized output %q", name)
		}
		seen[name] = true
		o := snapshot.BundleOutput{Path: name, Bytes: raw.Bytes, EntryPoint: raw.EntryPoint, Inputs: []snapshot.Contribution{}, Imports: []snapshot.Import{}}
		if o.EntryPoint != "" {
			o.EntryPoint = snapshot.CleanPath(o.EntryPoint)
		}
		contributions := make(map[string]int64)
		for _, input := range keys(raw.Inputs) {
			id, n := snapshot.CleanPath(input), raw.Inputs[input].BytesInOutput
			if previous, exists := contributions[id]; exists && previous != n {
				return nil, fmt.Errorf("conflicting contribution %q in output %q", id, name)
			}
			contributions[id] = n
		}
		for _, input := range keys(contributions) {
			o.Inputs = append(o.Inputs, snapshot.Contribution{Input: input, Bytes: contributions[input]})
		}
		for _, imp := range raw.Imports {
			p := imp.Path
			if !imp.External {
				p = snapshot.CleanPath(p)
			}
			o.Imports = append(o.Imports, snapshot.Import{Path: p, Dynamic: imp.Kind == "dynamic-import", External: imp.External, Asset: imp.Kind == "file-loader"})
		}
		slices.SortFunc(o.Imports, func(a, b snapshot.Import) int {
			if n := cmp.Compare(a.Path, b.Path); n != 0 {
				return n
			}
			if a.Dynamic != b.Dynamic {
				if a.Dynamic {
					return 1
				}
				return -1
			}
			if a.External != b.External {
				if a.External {
					return 1
				}
				return -1
			}
			if a.Asset != b.Asset {
				if a.Asset {
					return 1
				}
				return -1
			}
			return 0
		})
		s.Outputs = append(s.Outputs, o)
	}
	slices.SortFunc(s.Outputs, func(a, b snapshot.BundleOutput) int { return cmp.Compare(a.Path, b.Path) })
	return s, nil
}
