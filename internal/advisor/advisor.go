// Package advisor evaluates bundle graphs to propose actionable optimization recommendations.
package advisor

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/graph"
	"bundlecheck/internal/snapshot"
)

type Suggestion struct {
	Rule        string   `json:"rule"`
	Severity    string   `json:"severity"` // "HIGH", "MEDIUM", "LOW"
	Title       string   `json:"title"`
	Target      string   `json:"target"`
	File        string   `json:"file,omitempty"`
	Savings     int64    `json:"savingsBytes"`
	SavingsGzip int64    `json:"savingsGzipBytes"`
	Description string   `json:"description"`
	Action      string   `json:"action"`
}

type AdvisorResult struct {
	SchemaVersion         string       `json:"schemaVersion"`
	ToolVersion           string       `json:"toolVersion"`
	Command               string       `json:"command"`
	Suggestions           []Suggestion `json:"suggestions"`
	TotalPotentialSavings int64        `json:"totalPotentialSavings"`
}

type AdvisorOptions struct {
	MinSavings     int64
	SeverityFilter string
}

// Analyze evaluates the bundle snapshot and returns prioritized optimization suggestions.
func Analyze(s *snapshot.BundleSnapshot, opts AdvisorOptions) *AdvisorResult {
	res := &AdvisorResult{
		SchemaVersion: "1",
		ToolVersion:   analysis.ToolVersion,
		Command:       "suggest",
		Suggestions:   []Suggestion{},
	}
	if s == nil {
		return res
	}

	minSavings := opts.MinSavings
	if minSavings <= 0 {
		minSavings = 1024 // 1 KB default
	}

	// Rule 1: Heavy Initial Third-Party Utilities
	frameworkPackages := map[string]bool{
		"@angular/core":             true,
		"@angular/common":           true,
		"@angular/platform-browser": true,
		"@angular/router":           true,
		"@angular/compiler":         true,
		"rxjs":                      true,
		"zone.js":                   true,
		"tslib":                     true,
	}

	for _, p := range s.Packages {
		if p.InitialBytes < minSavings || frameworkPackages[p.Name] {
			continue
		}

		severity := "MEDIUM"
		if p.InitialBytes >= 40*1024 {
			severity = "HIGH"
		} else if p.InitialBytes < 5*1024 {
			severity = "LOW"
		}

		traceRes, _ := graph.TracePackage(s, p.Name, true, 1)
		importerFile := ""
		if traceRes != nil && len(traceRes.Chains) > 0 && len(traceRes.Chains[0].Path) > 1 {
			// Second to last in chain is the importer
			importerFile = traceRes.Chains[0].Path[len(traceRes.Chains[0].Path)-2]
		}

		res.Suggestions = append(res.Suggestions, Suggestion{
			Rule:        "heavy-initial-package",
			Severity:    severity,
			Title:       "Move '" + p.Name + "' behind a dynamic import",
			Target:      p.Name,
			File:        importerFile,
			Savings:     p.InitialBytes,
			SavingsGzip: p.InitialGzipBytes,
			Description: "Package '" + p.Name + "' is bundled in initial JS. If not critical for first paint, dynamic loading can directly reduce initial bundle size.",
			Action:      "Replace static 'import ... from \"" + p.Name + "\"' with dynamic 'const lib = await import(\"" + p.Name + "\")'.",
		})
	}

	// Rule 2: Eager Route / Page Components in Initial Outputs
	for _, o := range s.Outputs {
		if !o.Initial {
			continue
		}
		for _, c := range o.Inputs {
			if c.Bytes < minSavings {
				continue
			}
			baseName := strings.ToLower(path.Base(c.Input))
			isRouteCandidate := (strings.Contains(baseName, "page") || strings.Contains(baseName, "dialog") ||
				strings.Contains(baseName, "modal") || strings.Contains(baseName, "detail") ||
				strings.Contains(baseName, "dashboard") || strings.Contains(baseName, "feature")) &&
				!strings.HasSuffix(baseName, ".routes.ts") && !strings.HasSuffix(baseName, ".config.ts")

			if isRouteCandidate {
				severity := "MEDIUM"
				if c.Bytes >= 20*1024 {
					severity = "HIGH"
				}

				res.Suggestions = append(res.Suggestions, Suggestion{
					Rule:        "eager-feature-component",
					Severity:    severity,
					Title:       "Lazy-load feature component '" + path.Base(c.Input) + "'",
					Target:      c.Input,
					File:        c.Input,
					Savings:     c.Bytes,
					SavingsGzip: int64(float64(c.Bytes) * 0.32),
					Description: "Component '" + c.Input + "' is eagerly bundled into initial JS.",
					Action:      "Use 'loadComponent: () => import(\"./" + c.Input + "\")' in routes or wrap in '@defer (on viewport)'.",
				})
			}
		}
	}

	// Rule 3: Packages Split Across Initial and Lazy Chunks
	for _, p := range s.Packages {
		if p.InitialBytes >= minSavings && p.LazyBytes >= minSavings && !frameworkPackages[p.Name] {
			res.Suggestions = append(res.Suggestions, Suggestion{
				Rule:        "split-package",
				Severity:    "LOW",
				Title:       "Consolidate package usage for '" + p.Name + "'",
				Target:      p.Name,
				Savings:     p.InitialBytes,
				SavingsGzip: p.InitialGzipBytes,
				Description: "Package '" + p.Name + "' is included in both initial and lazy chunks.",
				Action:      "Ensure lazy features import from shared services rather than importing library modules directly.",
			})
		}
	}

	// Filter by severity if requested
	if opts.SeverityFilter != "" && opts.SeverityFilter != "ALL" {
		targetSev := strings.ToUpper(opts.SeverityFilter)
		filtered := []Suggestion{}
		for _, sugg := range res.Suggestions {
			if strings.ToUpper(sugg.Severity) == targetSev {
				filtered = append(filtered, sugg)
			}
		}
		res.Suggestions = filtered
	}

	// Sort suggestions by potential savings descending
	slices.SortFunc(res.Suggestions, func(a, b Suggestion) int {
		if n := cmp.Compare(b.Savings, a.Savings); n != 0 {
			return n
		}
		return cmp.Compare(a.Title, b.Title)
	})

	var totalSavings int64
	for _, s := range res.Suggestions {
		totalSavings += s.Savings
	}
	res.TotalPotentialSavings = totalSavings

	return res
}
