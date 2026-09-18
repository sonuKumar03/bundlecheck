// Package advisor evaluates bundle graphs to propose actionable optimization recommendations.
package advisor

import (
	"cmp"
	"fmt"
	"path"
	"slices"
	"strings"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/graph"
	"bundlecheck/internal/snapshot"
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

	// Pre-build indexed graph once for the entire analysis
	g := graph.NewGraph(s)

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
		topo := classifyTopology(chain)
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
					Title:       fmt.Sprintf("Deduplicate bundled package '%s' (%d copies found)", pkgName, len(roots)),
					Target:      pkgName,
					Savings:     estSavings,
					SavingsGzip: int64(float64(estSavings) * 0.32),
					Description: fmt.Sprintf("Multiple distinct copies of '%s' contribute to initial JS; deduplication savings are an estimate.", pkgName),
					Action:      "Run 'npm dedupe' or align package version constraints in package.json.",
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

	return res
}

type importerContext struct {
	role         string // "root-bootstrap", "route-component", "general-module"
	importerFile string
	routeFile    string
}

func classifyTopology(chain []string) importerContext {
	ctx := importerContext{
		role: "general-module",
	}
	if len(chain) < 2 {
		return ctx
	}

	ctx.importerFile = chain[len(chain)-2]

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
		title = fmt.Sprintf("Review root provider or module import for '%s'", pkgName)
		desc = fmt.Sprintf("Package '%s' is imported directly by root bootstrap (%s) and bundled into initial JS.", pkgName, topo.importerFile)
		action = "If not required for first paint, consider async providers (e.g. provide...Async()), lazy initialization, or scoping to feature routes."

	case "route-component":
		title = fmt.Sprintf("Lazy-load route component '%s' to defer '%s'", path.Base(topo.importerFile), pkgName)
		if topo.routeFile != "" && topo.routeFile != topo.importerFile {
			desc = fmt.Sprintf("Package '%s' is pulled into initial JS because '%s' is eagerly imported via '%s'.", pkgName, topo.importerFile, topo.routeFile)
		} else {
			desc = fmt.Sprintf("Package '%s' is pulled into initial JS because '%s' is eagerly imported.", pkgName, topo.importerFile)
		}
		action = fmt.Sprintf("Lazy-load the parent route (e.g. 'loadComponent: () => import(...)') or wrap in an '@defer' block to move '%s' to a lazy chunk.", pkgName)

	default: // "general-module"
		if topo.importerFile != "" {
			title = fmt.Sprintf("De-couple or lazy-load '%s' in %s", pkgName, path.Base(topo.importerFile))
			desc = fmt.Sprintf("Package '%s' contributes to initial JS via %s. If not needed during initial render, defer its loading.", pkgName, topo.importerFile)
			action = "Consider dynamic import ('const ... = await import(...)'), template '@defer' block, or tree-shakable subpath imports if only used for specific user interactions."
		} else {
			title = fmt.Sprintf("Move '%s' behind dynamic loading", pkgName)
			desc = fmt.Sprintf("Package '%s' is bundled in initial JS. If not critical for first paint, dynamic loading can directly reduce initial bundle size.", pkgName)
			action = "Consider dynamic import ('await import(...)'), '@defer', or moving non-critical logic to lazy routes."
		}
	}
	return title, desc, action
}

