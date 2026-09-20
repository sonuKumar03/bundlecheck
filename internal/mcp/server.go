package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mcpspec "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"bundlecheck/internal/advisor"
	"bundlecheck/internal/analysis"
	"bundlecheck/internal/baseline"
	"bundlecheck/internal/budget"
	"bundlecheck/internal/build"
	"bundlecheck/internal/comparison"
	"bundlecheck/internal/compression"
	"bundlecheck/internal/config"
	"bundlecheck/internal/discovery"
	"bundlecheck/internal/graph"
	"bundlecheck/internal/snapshot"
	"bundlecheck/internal/workspace"
)

// ServerVersion references analysis.ToolVersion as the single source of truth.
var ServerVersion = analysis.ToolVersion

// NewServer creates and initializes a bundlecheck MCP server with all tools and resources.
func NewServer() *server.MCPServer {
	s := server.NewMCPServer(
		"bundlecheck",
		ServerVersion,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(false, false),
		server.WithInstructions("BundleCheck MCP server provides Angular bundle inspection, size budget validation, and dependency tracing for AI coding agents."),
	)

	registerTools(s)
	registerResources(s)

	return s
}

func registerTools(s *server.MCPServer) {
	// 1. bundle_summary
	s.AddTool(mcpspec.NewTool(
		"bundle_summary",
		mcpspec.WithDescription("Analyze and summarize Angular esbuild bundle sizes (initial JS, lazy JS, total JS) and npm package contributors."),
		mcpspec.WithString("path", mcpspec.Description("Optional directory or stats.json file path. Defaults to current working directory.")),
		mcpspec.WithString("project", mcpspec.Description("Optional project name in a multi-app Nx or Angular workspace (e.g. 'portal').")),
		mcpspec.WithInteger("top", mcpspec.Description("Max number of top packages to include in summary. Defaults to 10.")),
		mcpspec.WithString("filter", mcpspec.Description("Optional substring to filter package names.")),
	), handleSummary)

	// 2. bundle_why
	s.AddTool(mcpspec.NewTool(
		"bundle_why",
		mcpspec.WithDescription("Trace why an npm package or module is included in the bundle, showing static/dynamic import paths from entrypoints."),
		mcpspec.WithString("package", mcpspec.Required(), mcpspec.Description("Name of the package or module to trace (e.g. 'lodash' or 'moment').")),
		mcpspec.WithString("path", mcpspec.Description("Optional directory or stats.json file path.")),
		mcpspec.WithString("project", mcpspec.Description("Optional project name in a multi-app workspace.")),
		mcpspec.WithBoolean("initial_only", mcpspec.Description("If true, only trace import paths leading to initial (startup) JS bundles.")),
		mcpspec.WithInteger("max_chains", mcpspec.Description("Max number of import chains to return. Defaults to 5.")),
	), handleWhy)

	// 3. bundle_suggest
	s.AddTool(mcpspec.NewTool(
		"bundle_suggest",
		mcpspec.WithDescription("Analyze bundle contributors and provide actionable optimization recommendations (e.g., heavy packages, duplicate libraries, lighter alternatives)."),
		mcpspec.WithString("path", mcpspec.Description("Optional directory or stats.json file path.")),
		mcpspec.WithString("project", mcpspec.Description("Optional project name in a multi-app workspace.")),
		mcpspec.WithInteger("min_savings", mcpspec.Description("Minimum potential byte savings to report. Defaults to 1024.")),
	), handleSuggest)

	// 4. bundle_check
	s.AddTool(mcpspec.NewTool(
		"bundle_check",
		mcpspec.WithDescription("Validate bundle sizes and rules against defined performance budgets (e.g., max initial size, disallowed packages)."),
		mcpspec.WithString("path", mcpspec.Description("Optional directory or stats.json file path.")),
		mcpspec.WithString("project", mcpspec.Description("Optional project name in a multi-app workspace.")),
		mcpspec.WithString("max_initial", mcpspec.Description("Maximum initial JS budget (e.g. '500KB', '1.5MB').")),
		mcpspec.WithString("max_total", mcpspec.Description("Maximum total JS budget (e.g. '2MB').")),
		mcpspec.WithArray("disallowed_packages", mcpspec.Description("List of package names that are forbidden from appearing in initial JS.")),
	), handleCheck)

	// 5. bundle_measure
	s.AddTool(mcpspec.NewTool(
		"bundle_measure",
		mcpspec.WithDescription("Compare current bundle build against a saved baseline snapshot and calculate size deltas."),
		mcpspec.WithString("baseline", mcpspec.Required(), mcpspec.Description("Name of saved baseline snapshot or file path to baseline JSON.")),
		mcpspec.WithString("path", mcpspec.Description("Optional directory or stats.json file path for the current build.")),
		mcpspec.WithString("project", mcpspec.Description("Optional project name in a multi-app workspace.")),
		mcpspec.WithString("max_initial_delta", mcpspec.Description("Maximum allowed increase in initial JS (e.g. '50KB', '0B').")),
	), handleMeasure)

	// 6. workspace_summary
	s.AddTool(mcpspec.NewTool(
		"workspace_summary",
		mcpspec.WithDescription("Inspect an Nx or Angular multi-app monorepo workspace, comparing all applications, shared library costs, and cross-app duplicates."),
		mcpspec.WithString("root", mcpspec.Description("Root directory of the monorepo workspace. Defaults to current directory.")),
		mcpspec.WithArray("projects", mcpspec.Description("Optional list of specific projects to analyze.")),
	), handleWorkspaceSummary)
}

func registerResources(s *server.MCPServer) {
	s.AddResource(
		mcpspec.NewResource(
			"bundlecheck://rules",
			"BundleCheck Optimization Rules",
			mcpspec.WithResourceDescription("Standard bundle optimization rules and modern replacement guidelines for common heavy packages."),
			mcpspec.WithMIMEType("application/json"),
		),
		func(ctx context.Context, request mcpspec.ReadResourceRequest) ([]mcpspec.ResourceContents, error) {
			rules := map[string]any{
				"recommendations": []map[string]string{
					{"package": "moment", "replacement": "date-fns or native Intl/Temporal", "reason": "Moment is large and not tree-shakeable"},
					{"package": "lodash", "replacement": "lodash-es or targeted imports (lodash/cloneDeep)", "reason": "CommonJS lodash pulls in full library"},
					{"package": "exceljs", "replacement": "dynamic import() in user-action handler", "reason": "Heavy spreadsheet library in initial bundle"},
					{"package": "pdfjs-dist", "replacement": "dynamic import() or lazy component", "reason": "Large PDF engine in initial bundle"},
				},
				"defaultBudgets": map[string]string{
					"initial_js_recommended": "500KB",
					"initial_js_warning":     "1MB",
					"initial_js_critical":    "2MB",
				},
			}
			data, _ := json.MarshalIndent(rules, "", "  ")
			return []mcpspec.ResourceContents{
				mcpspec.TextResourceContents{
					URI:      "bundlecheck://rules",
					MIMEType: "application/json",
					Text:     string(data),
				},
			}, nil
		},
	)
}

func resolveArtifacts(path, project string) (string, string, error) {
	searchDir := path
	if searchDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("get working directory: %w", err)
		}
		searchDir = wd
	}

	// If path points directly to a file (like stats.json)
	if fi, err := os.Stat(searchDir); err == nil && !fi.IsDir() {
		dir := filepath.Dir(searchDir)
		if s, d, err := discovery.Locate(dir, project); err == nil {
			return s, d, nil
		}
		browserSub := filepath.Join(dir, "browser")
		if bi, err := os.Stat(browserSub); err == nil && bi.IsDir() {
			return searchDir, browserSub, nil
		}
		return searchDir, dir, nil
	}

	return discovery.Locate(searchDir, project)
}

func handleSummary(ctx context.Context, req mcpspec.CallToolRequest) (*mcpspec.CallToolResult, error) {
	path := req.GetString("path", "")
	project := req.GetString("project", "")
	top := req.GetInt("top", 10)
	filter := req.GetString("filter", "")

	statsFile, distDir, err := resolveArtifacts(path, project)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to resolve build artifacts: %v", err)), nil
	}

	snap, err := build.Load(statsFile, distDir)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to load build: %v", err)), nil
	}

	res, err := analysis.Analyze(snap)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to analyze build: %v", err)), nil
	}
	compression.AttachCompression(snap, distDir)
	res.Summary = snap.Totals
	res.Packages = snap.Packages

	if filter != "" {
		filtered := []snapshot.Package{}
		for _, p := range res.Packages {
			if strings.Contains(strings.ToLower(p.Name), strings.ToLower(filter)) {
				filtered = append(filtered, p)
			}
		}
		res.Packages = filtered
	}

	if top > 0 && len(res.Packages) > top {
		res.Packages = res.Packages[:top]
	}

	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
	}

	return mcpspec.NewToolResultText(string(data)), nil
}

func handleWhy(ctx context.Context, req mcpspec.CallToolRequest) (*mcpspec.CallToolResult, error) {
	pkgName, err := req.RequireString("package")
	if err != nil {
		return mcpspec.NewToolResultError("Missing required parameter: package"), nil
	}
	path := req.GetString("path", "")
	project := req.GetString("project", "")
	initialOnly := req.GetBool("initial_only", false)
	maxChains := req.GetInt("max_chains", 5)

	statsFile, distDir, err := resolveArtifacts(path, project)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to resolve build artifacts: %v", err)), nil
	}

	snap, err := build.Load(statsFile, distDir)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to load build: %v", err)), nil
	}

	whyResult, err := graph.TracePackage(snap, pkgName, initialOnly, maxChains)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Trace error: %v", err)), nil
	}

	data, err := json.MarshalIndent(whyResult, "", "  ")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
	}

	return mcpspec.NewToolResultText(string(data)), nil
}

func handleSuggest(ctx context.Context, req mcpspec.CallToolRequest) (*mcpspec.CallToolResult, error) {
	path := req.GetString("path", "")
	project := req.GetString("project", "")
	minSavings := req.GetInt("min_savings", 1024)

	statsFile, distDir, err := resolveArtifacts(path, project)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to resolve build artifacts: %v", err)), nil
	}

	snap, err := build.Load(statsFile, distDir)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to load build: %v", err)), nil
	}

	if _, err := analysis.Analyze(snap); err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to analyze build: %v", err)), nil
	}
	compression.AttachCompression(snap, distDir)

	suggestions := advisor.Analyze(snap, advisor.AdvisorOptions{
		MinSavings: int64(minSavings),
	})

	data, err := json.MarshalIndent(suggestions, "", "  ")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
	}

	return mcpspec.NewToolResultText(string(data)), nil
}

func handleCheck(ctx context.Context, req mcpspec.CallToolRequest) (*mcpspec.CallToolResult, error) {
	path := req.GetString("path", "")
	project := req.GetString("project", "")
	maxInitial := req.GetString("max_initial", "")
	maxTotal := req.GetString("max_total", "")
	disallowed := req.GetStringSlice("disallowed_packages", nil)

	statsFile, distDir, err := resolveArtifacts(path, project)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to resolve build artifacts: %v", err)), nil
	}

	snap, err := build.Load(statsFile, distDir)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to load build: %v", err)), nil
	}

	res, err := analysis.Analyze(snap)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to analyze build: %v", err)), nil
	}

	var limits budget.Limits
	if maxInitial != "" {
		b, err := budget.ParseBytes(maxInitial)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Invalid max_initial: %v", err)), nil
		}
		limits.MaxInitial = &b
	}
	if maxTotal != "" {
		b, err := budget.ParseBytes(maxTotal)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Invalid max_total: %v", err)), nil
		}
		limits.MaxTotal = &b
	}

	checkResult := budget.CheckSummary(res.Summary, limits)
	if len(disallowed) > 0 {
		cfg := &config.Config{
			Rules: config.ConfigRules{
				DisallowPackages: disallowed,
			},
		}
		ruleViolations := cfg.CheckRules(res)
		if len(ruleViolations) > 0 {
			checkResult.Passed = false
			checkResult.Violations = append(checkResult.Violations, ruleViolations...)
		}
	}

	data, err := json.MarshalIndent(checkResult, "", "  ")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
	}

	return mcpspec.NewToolResultText(string(data)), nil
}

func handleMeasure(ctx context.Context, req mcpspec.CallToolRequest) (*mcpspec.CallToolResult, error) {
	baseName, err := req.RequireString("baseline")
	if err != nil {
		return mcpspec.NewToolResultError("Missing required parameter: baseline"), nil
	}
	path := req.GetString("path", "")
	project := req.GetString("project", "")
	maxInitialDeltaStr := req.GetString("max_initial_delta", "")

	statsFile, distDir, err := resolveArtifacts(path, project)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to resolve build artifacts: %v", err)), nil
	}

	currentSnap, err := build.Load(statsFile, distDir)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to load build: %v", err)), nil
	}

	baseResult, err := baseline.Load(baseName)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to load baseline %q: %v", baseName, err)), nil
	}

	currentRes, err := analysis.Analyze(currentSnap)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to analyze current build: %v", err)), nil
	}
	compression.AttachCompression(currentSnap, distDir)
	currentRes.Summary = currentSnap.Totals
	currentRes.Packages = currentSnap.Packages

	cmpResult := comparison.Compare(baseResult, currentRes)

	if maxInitialDeltaStr != "" {
		limitBytes, err := budget.ParseBytes(maxInitialDeltaStr)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Invalid max_initial_delta: %v", err)), nil
		}
		limits := budget.Limits{
			MaxInitialDelta: &limitBytes,
		}
		check := budget.CheckComparison(cmpResult, limits)
		if !check.Passed {
			return mcpspec.NewToolResultError(fmt.Sprintf("Budget check failed: %s", check.Violations[0].Message)), nil
		}
	}

	data, err := json.MarshalIndent(cmpResult, "", "  ")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
	}

	return mcpspec.NewToolResultText(string(data)), nil
}

func handleWorkspaceSummary(ctx context.Context, req mcpspec.CallToolRequest) (*mcpspec.CallToolResult, error) {
	root := req.GetString("root", "")
	projects := req.GetStringSlice("projects", nil)

	wsResult, err := workspace.Analyze(ctx, workspace.AnalyzeOptions{
		Root:            root,
		ExplicitRoot:    root != "",
		Projects:        projects,
		WithCompression: true,
	})
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Workspace analysis error: %v", err)), nil
	}

	data, err := json.MarshalIndent(wsResult, "", "  ")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
	}

	return mcpspec.NewToolResultText(string(data)), nil
}
