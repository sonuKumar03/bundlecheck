// Package analysis computes byte facts from framework-independent snapshots.
package analysis

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

// ToolVersion is the bundlecheck version, which can be overridden at compile time via:
// go build -ldflags "-X bundlecheck/internal/analysis.ToolVersion=vX.Y.Z"
var ToolVersion = "0.4.0"

type AnalysisResult struct {
	SchemaVersion string             `json:"schemaVersion"`
	ToolVersion   string             `json:"toolVersion"`
	Command       string             `json:"command"`
	Summary       snapshot.Totals    `json:"summary"`
	Packages      []snapshot.Package `json:"packages"`
}

func Analyze(s *snapshot.BundleSnapshot) (*AnalysisResult, error) {
	if s == nil {
		return nil, fmt.Errorf("cannot analyze nil snapshot")
	}
	r := &AnalysisResult{SchemaVersion: s.SchemaVersion, ToolVersion: ToolVersion, Command: "summary", Packages: []snapshot.Package{}}
	packages := make(map[string]snapshot.Package)
	seenOutputs := make(map[string]bool)
	for _, o := range s.Outputs {
		id := snapshot.CleanPath(o.Path)
		if seenOutputs[id] {
			return nil, fmt.Errorf("duplicate output %q", id)
		}
		seenOutputs[id] = true
		if !snapshot.IsJavaScript(id) {
			continue
		}
		if err := add(&r.Summary.TotalJS, o.Bytes); err != nil {
			return nil, fmt.Errorf("output %q: %w", id, err)
		}
		total := &r.Summary.LazyJS
		if o.Initial {
			total = &r.Summary.InitialJS
		}
		if err := add(total, o.Bytes); err != nil {
			return nil, err
		}
		seenInputs := make(map[string]int64)
		for _, c := range o.Inputs {
			input := snapshot.CleanPath(c.Input)
			if c.Bytes < 0 {
				return nil, fmt.Errorf("negative contribution for %q", input)
			}
			if previous, exists := seenInputs[input]; exists {
				if previous != c.Bytes {
					return nil, fmt.Errorf("conflicting contribution %q in output %q", input, id)
				}
				continue
			}
			seenInputs[input] = c.Bytes
			name, ok := PackageName(input)
			if !ok || c.Bytes == 0 {
				continue
			}
			p := packages[name]
			p.Name = name
			bucket := &p.LazyBytes
			if o.Initial {
				bucket = &p.InitialBytes
			}
			if err := add(bucket, c.Bytes); err != nil {
				return nil, fmt.Errorf("package %q: %w", name, err)
			}
			if err := add(&p.TotalBytes, c.Bytes); err != nil {
				return nil, fmt.Errorf("package %q: %w", name, err)
			}
			packages[name] = p
		}
	}
	for _, p := range packages {
		r.Packages = append(r.Packages, p)
	}
	slices.SortFunc(r.Packages, func(a, b snapshot.Package) int {
		if n := cmp.Compare(b.InitialBytes, a.InitialBytes); n != 0 {
			return n
		}
		return cmp.Compare(a.Name, b.Name)
	})
	s.Totals = r.Summary
	s.Packages = r.Packages
	return r, nil
}

func add(total *int64, n int64) error {
	if n < 0 {
		return fmt.Errorf("byte count must be nonnegative")
	}
	if n > math.MaxInt64-*total {
		return fmt.Errorf("byte total exceeds int64")
	}
	*total += n
	return nil
}
