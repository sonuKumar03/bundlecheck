// Package analysis computes byte facts from framework-independent snapshots.
package analysis

import (
	"cmp"
	"fmt"
	"math"
	"path"
	"slices"

	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

// ToolVersion is the bundlecheck version, which can be overridden at compile time via:
// go build -ldflags "-X bundlecheck/internal/analysis.ToolVersion=vX.Y.Z"
var ToolVersion = "0.4.2"

type SourceContribution struct {
	Name         string `json:"name"`
	InitialBytes int64  `json:"initialBytes"`
	LazyBytes    int64  `json:"lazyBytes"`
	TotalBytes   int64  `json:"totalBytes"`
}

type AnalysisResult struct {
	SchemaVersion string               `json:"schemaVersion"`
	ToolVersion   string               `json:"toolVersion"`
	Command       string               `json:"command"`
	Summary       snapshot.Totals      `json:"summary"`
	Packages      []snapshot.Package   `json:"packages"`
	Sources       []SourceContribution `json:"sources,omitempty"`
}

// SourceDir returns the logical component or source directory for an application file.
func SourceDir(input string) string {
	cleaned := snapshot.CleanPath(input)
	dir := path.Dir(cleaned)
	if dir == "." || dir == "" {
		return cleaned
	}
	return dir
}

func Analyze(s *snapshot.BundleSnapshot) (*AnalysisResult, error) {
	if s == nil {
		return nil, fmt.Errorf("cannot analyze nil snapshot")
	}
	r := &AnalysisResult{
		SchemaVersion: s.SchemaVersion,
		ToolVersion:   ToolVersion,
		Command:       "summary",
		Packages:      []snapshot.Package{},
		Sources:       []SourceContribution{},
	}
	packages := make(map[string]snapshot.Package)
	sources := make(map[string]SourceContribution)
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
			if c.Bytes == 0 {
				continue
			}
			if name, ok := PackageName(input); ok {
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
			} else {
				dir := SourceDir(input)
				src := sources[dir]
				src.Name = dir
				bucket := &src.LazyBytes
				if o.Initial {
					bucket = &src.InitialBytes
				}
				if err := add(bucket, c.Bytes); err != nil {
					return nil, fmt.Errorf("source %q: %w", dir, err)
				}
				if err := add(&src.TotalBytes, c.Bytes); err != nil {
					return nil, fmt.Errorf("source %q: %w", dir, err)
				}
				sources[dir] = src
			}
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
	for _, src := range sources {
		r.Sources = append(r.Sources, src)
	}
	slices.SortFunc(r.Sources, func(a, b SourceContribution) int {
		if n := cmp.Compare(b.InitialBytes, a.InitialBytes); n != 0 {
			return n
		}
		if n := cmp.Compare(b.TotalBytes, a.TotalBytes); n != 0 {
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
