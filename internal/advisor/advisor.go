// Package advisor evaluates bundle graphs to propose actionable optimization recommendations.
package advisor

import (
	"cmp"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/analysis"
	"github.com/sonuKumar03/bundleradar/internal/graph"
	"github.com/sonuKumar03/bundleradar/internal/snapshot"
)

type Suggestion struct {
	Rule        string `json:"rule"`
	Severity    string `json:"severity"` // "HIGH", "MEDIUM", "LOW"
	Title       string `json:"title"`
	Target      string `json:"target"`
	File        string `json:"file,omitempty"`
	Savings     int64  `json:"savingsBytes"`
	SavingsGzip int64  `json:"savingsGzipBytes"`
	Description string `json:"description"`
	Action      string `json:"action"`
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
	res, _ := AnalyzeWithEntry(s, opts, "")
	return res
}

// AnalyzeWithEntry evaluates the bundle snapshot scoped to an entry and returns prioritized optimization suggestions.
func AnalyzeWithEntry(s *snapshot.BundleSnapshot, opts AdvisorOptions, entry string) (*AdvisorResult, error) {
	res := &AdvisorResult{
		SchemaVersion: "1",
		ToolVersion:   analysis.ToolVersion,
		Command:       "suggest",
		Suggestions:   []Suggestion{},
	}
	if s == nil {
		return res, nil
	}

	minSavings := opts.MinSavings
	if minSavings <= 0 {
		minSavings = 1024 // 1 KB default
	}

	// Pre-build indexed graph once for the entire analysis
	g, err := graph.NewGraphWithEntry(s, entry)
	if err != nil {
		return nil, err
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

		if opts.SeverityFilter != "" && opts.SeverityFilter != "ALL" {
			if !strings.EqualFold(severity, opts.SeverityFilter) {
				continue
			}
		}

		traceRes, _ := g.TracePackage(p.Name, true, 1)
		var chain []string
		if traceRes != nil && len(traceRes.Chains) > 0 {
			chain = traceRes.Chains[0].Path
		}
		topo := classifyTopology(chain, p.Name)
		title, desc, action := generatePackageSuggestion(p.Name, topo)

		res.Suggestions = append(res.Suggestions, Suggestion{
			Rule:        "heavy-initial-package",
			Severity:    severity,
			Title:       title,
			Target:      p.Name,
			File:        topo.importerFile,
			Savings:     p.InitialBytes,
			SavingsGzip: p.InitialGzipBytes,
			Description: desc,
			Action:      action,
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
					Title:       "Eager component '" + path.Base(c.Input) + "' bundled in initial JS",
					Target:      c.Input,
					File:        c.Input,
					Savings:     c.Bytes,
					SavingsGzip: int64(float64(c.Bytes) * 0.32),
					Description: "Component '" + c.Input + "' is eagerly bundled into initial JS.",
					Action:      "",
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
				Title:       "Package '" + p.Name + "' split across initial and lazy chunks",
				Target:      p.Name,
				Savings:     p.InitialBytes,
				SavingsGzip: p.InitialGzipBytes,
				Description: "Package '" + p.Name + "' is included in both initial and lazy chunks.",
				Action:      "",
			})
		}
	}

	// Rule 4: Distinct package copies contributing to initial JS.
	pkgRootDirs := make(map[string]map[string]int64)
	for _, output := range s.Outputs {
		if !output.Initial {
			continue
		}
		for _, contribution := range output.Inputs {
			p := snapshot.CleanPath(contribution.Input)
			pkgName, isPkg := analysis.PackageName(p)
			if !isPkg || contribution.Bytes <= 0 {
				continue
			}
			idx := strings.LastIndex(p, "node_modules/"+pkgName)
			pkgRoot := p[:idx+len("node_modules/"+pkgName)]
			if pkgRootDirs[pkgName] == nil {
				pkgRootDirs[pkgName] = make(map[string]int64)
			}
			pkgRootDirs[pkgName][pkgRoot] += contribution.Bytes
		}
	}

	for pkgName, roots := range pkgRootDirs {
		if len(roots) > 1 {
			var dupBytes int64
			var largestCopy int64
			for _, b := range roots {
				dupBytes += b
				largestCopy = max(largestCopy, b)
			}
			estSavings := dupBytes - largestCopy
			if estSavings >= minSavings {
				res.Suggestions = append(res.Suggestions, Suggestion{
					Rule:        "duplicate-package",
					Severity:    "MEDIUM",
					Title:       fmt.Sprintf("Duplicate package '%s' (%d copies found in initial JS)", pkgName, len(roots)),
					Target:      pkgName,
					Savings:     estSavings,
					SavingsGzip: int64(float64(estSavings) * 0.32),
					Description: fmt.Sprintf("Multiple distinct copies of '%s' contribute to initial JS.", pkgName),
					Action:      "",
				})
			}
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

	// Compute deduplicated total potential savings without overlapping double-counts
	claimedTargetSavings := make(map[string]int64)
	for _, sugg := range res.Suggestions {
		if currentClaim, exists := claimedTargetSavings[sugg.Target]; !exists || sugg.Savings > currentClaim {
			claimedTargetSavings[sugg.Target] = sugg.Savings
		}
	}

	var totalSavings int64
	for _, savings := range claimedTargetSavings {
		totalSavings += savings
	}
	if s.Totals.InitialJS > 0 && totalSavings > s.Totals.InitialJS {
		totalSavings = s.Totals.InitialJS
	}
	res.TotalPotentialSavings = totalSavings

	return res, nil
}

type importerContext struct {
	role         string // "root-bootstrap", "route-component", "general-module"
	importerFile string
	routeFile    string
}

func classifyTopology(chain []string, pkgName string) importerContext {
	ctx := importerContext{
		role: "general-module",
	}
	if len(chain) == 0 {
		return ctx
	}

	// 1. Find the last application file in chain (not in node_modules)
	var appImporter string
	for i := len(chain) - 1; i >= 0; i-- {
		p := chain[i]
		if !strings.Contains(p, "node_modules/") {
			appImporter = p
			break
		}
	}

	// 2. Find the direct external caller (not in pkgName)
	var directExternalCaller string
	for i := len(chain) - 1; i >= 0; i-- {
		p := chain[i]
		if pkg, isPkg := analysis.PackageName(p); !isPkg || pkg != pkgName {
			directExternalCaller = p
			break
		}
	}

	// Use application importer if available, otherwise direct external caller
	if appImporter != "" {
		ctx.importerFile = appImporter
	} else if directExternalCaller != "" {
		ctx.importerFile = directExternalCaller
	} else if len(chain) > 1 {
		ctx.importerFile = chain[len(chain)-2]
	}

	// Check if any module in the chain is a routing file
	for _, p := range chain {
		base := strings.ToLower(path.Base(p))
		if strings.Contains(base, "route") || strings.HasSuffix(base, ".routes.ts") || strings.HasSuffix(base, "-routing.module.ts") {
			ctx.routeFile = p
			break
		}
	}

	importerBase := strings.ToLower(path.Base(ctx.importerFile))

	// Check if the importer itself or route in chain indicates a route/page/feature component
	isRouteFeature := ctx.routeFile != "" ||
		strings.Contains(importerBase, "page") ||
		strings.Contains(importerBase, "dialog") ||
		strings.Contains(importerBase, "modal") ||
		strings.Contains(importerBase, "detail") ||
		strings.Contains(importerBase, "dashboard") ||
		strings.Contains(importerBase, "feature") ||
		strings.Contains(importerBase, "view") ||
		strings.Contains(importerBase, "screen")

	// If it's a component (and not root app.component.ts), treat as route/feature component
	if strings.Contains(importerBase, "component") && !strings.HasPrefix(importerBase, "app.component") {
		isRouteFeature = true
	}

	if isRouteFeature {
		ctx.role = "route-component"
		return ctx
	}

	// Check if importer is root bootstrap
	if isRootBootstrap(importerBase) {
		ctx.role = "root-bootstrap"
		return ctx
	}

	return ctx
}

func isRootBootstrap(base string) bool {
	base = strings.ToLower(base)
	return strings.HasSuffix(base, ".config.ts") ||
		strings.HasSuffix(base, ".config.server.ts") ||
		base == "main.ts" ||
		base == "main.server.ts" ||
		base == "bootstrap.ts" ||
		base == "polyfills.ts" ||
		(strings.HasSuffix(base, ".module.ts") && (strings.HasPrefix(base, "app.") || strings.HasPrefix(base, "root.")))
}

func generatePackageSuggestion(pkgName string, topo importerContext) (title, desc, action string) {
	switch topo.role {
	case "root-bootstrap":
		title = fmt.Sprintf("Initial JS package '%s' imported by root bootstrap", pkgName)
		desc = fmt.Sprintf("Package '%s' is bundled in initial JS because it is imported by root bootstrap '%s'.", pkgName, topo.importerFile)
		action = ""

	case "route-component":
		title = fmt.Sprintf("Initial JS package '%s' imported by eager component '%s'", pkgName, path.Base(topo.importerFile))
		if topo.routeFile != "" && topo.routeFile != topo.importerFile {
			desc = fmt.Sprintf("Package '%s' is bundled in initial JS because '%s' is imported by route definition '%s'.", pkgName, topo.importerFile, topo.routeFile)
		} else {
			desc = fmt.Sprintf("Package '%s' is bundled in initial JS because '%s' is statically imported.", pkgName, topo.importerFile)
		}
		action = ""

	default: // "general-module"
		if topo.importerFile != "" {
			title = fmt.Sprintf("Initial JS package '%s' imported by %s", pkgName, path.Base(topo.importerFile))
			desc = fmt.Sprintf("Package '%s' contributes to initial JS via '%s'.", pkgName, topo.importerFile)
		} else {
			title = fmt.Sprintf("Initial JS package '%s'", pkgName)
			desc = fmt.Sprintf("Package '%s' contributes to initial JS.", pkgName)
		}
		action = ""
	}
	return title, desc, action
}

