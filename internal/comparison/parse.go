package comparison

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/snapshot"
)

type wireTotals struct {
	Initial *int64 `json:"initialJs"`
	Lazy    *int64 `json:"lazyJs"`
	Total   *int64 `json:"totalJs"`
}

type wirePackage struct {
	Name    string `json:"name"`
	Initial *int64 `json:"initialBytes"`
	Lazy    *int64 `json:"lazyBytes"`
	Total   *int64 `json:"totalBytes"`
}

// Parse accepts summary schema 1, including unknown metadata fields. Pointer
// counts distinguish legitimate zero values from missing or null fields.
func Parse(path string) (*analysis.AnalysisResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read summary %q: %w", path, err)
	}
	r, err := parse(data)
	if err != nil {
		return nil, fmt.Errorf("summary %q: %w", path, err)
	}
	return r, nil
}

func parse(data []byte) (*analysis.AnalysisResult, error) {
	var wire struct {
		SchemaVersion string         `json:"schemaVersion"`
		ToolVersion   string         `json:"toolVersion"`
		Command       string         `json:"command"`
		Summary       *wireTotals    `json:"summary"`
		Packages      *[]wirePackage `json:"packages"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if wire.SchemaVersion != "1" {
		return nil, fmt.Errorf("unsupported schemaVersion %q: require 1", wire.SchemaVersion)
	}
	if wire.Command != "summary" {
		return nil, fmt.Errorf("command must be summary, got %q", wire.Command)
	}
	if strings.TrimSpace(wire.ToolVersion) == "" {
		return nil, fmt.Errorf("toolVersion is required")
	}
	if wire.Summary == nil {
		return nil, fmt.Errorf("summary must be a non-null object")
	}
	if wire.Packages == nil {
		return nil, fmt.Errorf("packages must be a non-null array")
	}
	b, err := counts(wire.Summary.Initial, wire.Summary.Lazy, wire.Summary.Total, [3]string{"initialJs", "lazyJs", "totalJs"})
	if err != nil {
		return nil, fmt.Errorf("summary: %w", err)
	}
	r := &analysis.AnalysisResult{SchemaVersion: wire.SchemaVersion, ToolVersion: wire.ToolVersion, Command: wire.Command,
		Summary: snapshot.Totals{InitialJS: b.InitialBytes, LazyJS: b.LazyBytes, TotalJS: b.TotalBytes}, Packages: []snapshot.Package{},
	}
	seen := make(map[string]bool)
	for i, p := range *wire.Packages {
		if strings.TrimSpace(p.Name) == "" {
			return nil, fmt.Errorf("packages[%d].name is required", i)
		}
		if seen[p.Name] {
			return nil, fmt.Errorf("duplicate package %q", p.Name)
		}
		seen[p.Name] = true
		b, err := counts(p.Initial, p.Lazy, p.Total, [3]string{"initialBytes", "lazyBytes", "totalBytes"})
		if err != nil {
			return nil, fmt.Errorf("package %q: %w", p.Name, err)
		}
		r.Packages = append(r.Packages, snapshot.Package{Name: p.Name, InitialBytes: b.InitialBytes, LazyBytes: b.LazyBytes, TotalBytes: b.TotalBytes})
	}
	return r, nil
}

func counts(initial, lazy, total *int64, names [3]string) (Bytes, error) {
	for i, n := range []*int64{initial, lazy, total} {
		if n == nil {
			return Bytes{}, fmt.Errorf("%s is required and must not be null", names[i])
		}
		if *n < 0 {
			return Bytes{}, fmt.Errorf("%s must be nonnegative", names[i])
		}
	}
	if *initial > math.MaxInt64-*lazy || *initial+*lazy != *total {
		return Bytes{}, fmt.Errorf("%s must equal initial + lazy without overflow", names[2])
	}
	return Bytes{*initial, *lazy, *total}, nil
}
