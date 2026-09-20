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

	"github.com/sonuKumar03/bundlecheck/internal/advisor"
	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/baseline"
	"github.com/sonuKumar03/bundlecheck/internal/budget"
	"github.com/sonuKumar03/bundlecheck/internal/build"
	"github.com/sonuKumar03/bundlecheck/internal/comparison"
	"github.com/sonuKumar03/bundlecheck/internal/compression"
	"github.com/sonuKumar03/bundlecheck/internal/config"
	"github.com/sonuKumar03/bundlecheck/internal/discovery"
	"github.com/sonuKumar03/bundlecheck/internal/graph"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
	"github.com/sonuKumar03/bundlecheck/internal/workspace"
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
		mcpspec.WithReadOnlyHintAnnotation(true),
		mcpspec.WithDestructiveHintAnnotation(false),
		mcpspec.WithIdempotentHintAnnotation(true),
		mcpspec.WithOpenWorldHintAnnotation(false),
		mcpspec.WithString("path", mcpspec.Description("Optional directory or stats.json file path. Defaults to current working directory.")),
		mcpspec.WithString("project", mcpspec.Description("Optional project name in a multi-app Nx or Angular workspace (e.g. 'portal').")),
		mcpspec.WithInteger("top", mcpspec.Description("Max number of top packages to include in summary. Defaults to 10.")),
		mcpspec.WithString("filter", mcpspec.Description("Optional substring to filter package names.")),
	), handleSummary)

	// 2. bundle_why
	s.AddTool(mcpspec.NewTool(
		"bundle_why",
		mcpspec.WithDescription("Trace why an npm package or module is included in the bundle, showing static/dynamic import paths from entrypoints."),
		mcpspec.WithReadOnlyHintAnnotation(true),
		mcpspec.WithDestructiveHintAnnotation(false),
		mcpspec.WithIdempotentHintAnnotation(true),
		mcpspec.WithOpenWorldHintAnnotation(false),
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
		mcpspec.WithReadOnlyHintAnnotation(true),
		mcpspec.WithDestructiveHintAnnotation(false),
		mcpspec.WithIdempotentHintAnnotation(true),
		mcpspec.WithOpenWorldHintAnnotation(false),
		mcpspec.WithString("path", mcpspec.Description("Optional directory or stats.json file path.")),
		mcpspec.WithString("project", mcpspec.Description("Optional project name in a multi-app workspace.")),
		mcpspec.WithInteger("min_savings", mcpspec.Description("Minimum potential byte savings to report. Defaults to 1024.")),
	), handleSuggest)

	// 4. bundle_check
	s.AddTool(mcpspec.NewTool(
		"bundle_check",
		mcpspec.WithDescription("Validate bundle sizes, regressions, and repository rules against defined performance budgets and .bundlecheck.yml."),
		mcpspec.WithReadOnlyHintAnnotation(true),
		mcpspec.WithDestructiveHintAnnotation(false),
		mcpspec.WithIdempotentHintAnnotation(true),
		mcpspec.WithOpenWorldHintAnnotation(false),
		mcpspec.WithString("path", mcpspec.Description("Optional directory or stats.json file path.")),
		mcpspec.WithString("project", mcpspec.Description("Optional project name in a multi-app workspace.")),
		mcpspec.WithString("config", mcpspec.Description("Optional path to .bundlecheck.yml configuration file.")),
		mcpspec.WithString("baseline", mcpspec.Description("Optional path to baseline summary JSON or saved baseline name for regression checks.")),
		mcpspec.WithString("max_initial", mcpspec.Description("Maximum initial JS budget (e.g. '500KB', '1.5MB').")),
		mcpspec.WithString("max_lazy", mcpspec.Description("Maximum lazy JS budget (e.g. '500KB', '1MB').")),
		mcpspec.WithString("max_total", mcpspec.Description("Maximum total JS budget (e.g. '2MB').")),
		mcpspec.WithString("max_initial_delta", mcpspec.Description("Maximum allowed increase in initial JS vs baseline (e.g. '50KB', '0B').")),
		mcpspec.WithString("max_total_delta", mcpspec.Description("Maximum allowed increase in total JS vs baseline (e.g. '100KB').")),
		mcpspec.WithArray("disallowed_packages", mcpspec.WithStringItems(), mcpspec.Description("List of package names that are forbidden from appearing anywhere in the bundle.")),
	), handleCheck)

	// 5. bundle_measure
	s.AddTool(mcpspec.NewTool(
		"bundle_measure",
		mcpspec.WithDescription("Compare current bundle build against a saved baseline snapshot and calculate size deltas."),
		mcpspec.WithReadOnlyHintAnnotation(true),
		mcpspec.WithDestructiveHintAnnotation(false),
		mcpspec.WithIdempotentHintAnnotation(true),
		mcpspec.WithOpenWorldHintAnnotation(false),
		mcpspec.WithString("baseline", mcpspec.Required(), mcpspec.Description("Name of saved baseline snapshot or file path to baseline JSON.")),
		mcpspec.WithString("path", mcpspec.Description("Optional directory or stats.json file path for the current build.")),
		mcpspec.WithString("project", mcpspec.Description("Optional project name in a multi-app workspace.")),
		mcpspec.WithString("max_initial_delta", mcpspec.Description("Maximum allowed increase in initial JS (e.g. '50KB', '0B').")),
		mcpspec.WithString("max_total_delta", mcpspec.Description("Maximum allowed increase in total JS (e.g. '50KB', '0B').")),
	), handleMeasure)

	// 6. workspace_summary
	s.AddTool(mcpspec.NewTool(
		"workspace_summary",
		mcpspec.WithDescription("Inspect an Nx or Angular multi-app monorepo workspace, comparing all applications, shared library costs, and cross-app duplicates. Fast-path static project.json parsing is used by default; set allow_nx_fallback to true to permit fallback to workspace-installed Nx CLI execution."),
		mcpspec.WithReadOnlyHintAnnotation(false),
		mcpspec.WithDestructiveHintAnnotation(false),
		mcpspec.WithIdempotentHintAnnotation(true),
		mcpspec.WithOpenWorldHintAnnotation(true),
		mcpspec.WithString("root", mcpspec.Description("Root directory of the monorepo workspace. Defaults to current directory.")),
		mcpspec.WithArray("projects", mcpspec.WithStringItems(), mcpspec.Description("Optional list of specific projects to analyze.")),
		mcpspec.WithBoolean("allow_nx_fallback", mcpspec.Description("Allow falling back to executing workspace-installed Nx CLI if static parsing cannot resolve the project graph. Defaults to false for security on untrusted workspaces.")),
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
	return discovery.Resolve("", path, "", project)
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

func parseStringSlice(args map[string]any, key string) ([]string, error) {
	val, ok := args[key]
	if !ok || val == nil {
		return nil, nil
	}
	switch v := val.(type) {
	case []string:
		return v, nil
	case []any:
		result := make([]string, len(v))
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("element %d in %s must be a string", i, key)
			}
			result[i] = s
		}
		return result, nil
	default:
		return nil, fmt.Errorf("parameter %s must be an array of strings", key)
	}
}

type measureResponse struct {
	*comparison.Result
	Budget *budget.CheckResult `json:"budget,omitempty"`
}

func handleCheck(ctx context.Context, req mcpspec.CallToolRequest) (*mcpspec.CallToolResult, error) {
	path := req.GetString("path", "")
	project := req.GetString("project", "")
	configPath := req.GetString("config", "")
	baselinePath := req.GetString("baseline", "")
	maxInitial := req.GetString("max_initial", "")
	maxLazy := req.GetString("max_lazy", "")
	maxTotal := req.GetString("max_total", "")
	maxInitialDelta := req.GetString("max_initial_delta", "")
	maxTotalDelta := req.GetString("max_total_delta", "")
	disallowed, err := parseStringSlice(req.GetArguments(), "disallowed_packages")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Invalid parameter: %v", err)), nil
	}

	statsFile, distDir, err := resolveArtifacts(path, project)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to resolve build artifacts: %v", err)), nil
	}

	// 1. Load config file if present or specified
	var cfg *config.Config
	if configPath != "" {
		cfg, err = config.LoadFile(configPath)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Failed to load config %q: %v", configPath, err)), nil
		}
	} else {
		searchDir := path
		if searchDir == "" {
			searchDir = filepath.Dir(statsFile)
		} else if fi, statErr := os.Stat(searchDir); statErr == nil && !fi.IsDir() {
			searchDir = filepath.Dir(searchDir)
		}
		cfg, _, err = config.FindAndLoad(searchDir)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Failed to load config: %v", err)), nil
		}
	}

	// 2. Parse Limits
	var limits budget.Limits
	if maxInitial != "" {
		b, err := budget.ParseBytes(maxInitial)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Invalid max_initial: %v", err)), nil
		}
		limits.MaxInitial = &b
	}
	if maxLazy != "" {
		b, err := budget.ParseBytes(maxLazy)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Invalid max_lazy: %v", err)), nil
		}
		limits.MaxLazy = &b
	}
	if maxTotal != "" {
		b, err := budget.ParseBytes(maxTotal)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Invalid max_total: %v", err)), nil
		}
		limits.MaxTotal = &b
	}
	if maxInitialDelta != "" {
		b, err := budget.ParseBytes(maxInitialDelta)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Invalid max_initial_delta: %v", err)), nil
		}
		limits.MaxInitialDelta = &b
	}
	if maxTotalDelta != "" {
		b, err := budget.ParseBytes(maxTotalDelta)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Invalid max_total_delta: %v", err)), nil
		}
		limits.MaxTotalDelta = &b
	}

	// 3. Apply config defaults to unset limits
	if cfg != nil {
		if err := cfg.ApplyToLimits(&limits); err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Failed to apply config limits: %v", err)), nil
		}
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

	var checkResult budget.CheckResult
	if limits.MaxInitialDelta != nil || limits.MaxTotalDelta != nil || baselinePath != "" {
		baseResult, err := baseline.Load(baselinePath)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Failed to load baseline %q: %v", baselinePath, err)), nil
		}
		compResult := comparison.CompareWithSnapshot(baseResult, res, snap)
		checkResult = budget.CheckComparison(compResult, limits)
		checkResult.Comparison = compResult
	} else {
		checkResult = budget.CheckSummary(res.Summary, limits)
		checkResult.Summary = &res.Summary
	}

	var allDisallowed []string
	if cfg != nil {
		allDisallowed = append(allDisallowed, cfg.Rules.DisallowPackages...)
	}
	allDisallowed = append(allDisallowed, disallowed...)
	if len(allDisallowed) > 0 {
		ruleCfg := &config.Config{
			Rules: config.ConfigRules{
				DisallowPackages: allDisallowed,
			},
		}
		ruleViolations := ruleCfg.CheckRules(res)
		if len(ruleViolations) > 0 {
			checkResult.Passed = false
			checkResult.Violations = append(checkResult.Violations, ruleViolations...)
		}
	}

	data, err := json.MarshalIndent(checkResult, "", "  ")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
	}

	toolRes := mcpspec.NewToolResultText(string(data))
	toolRes.IsError = !checkResult.Passed
	return toolRes, nil
}

func handleMeasure(ctx context.Context, req mcpspec.CallToolRequest) (*mcpspec.CallToolResult, error) {
	baseName, err := req.RequireString("baseline")
	if err != nil {
		return mcpspec.NewToolResultError("Missing required parameter: baseline"), nil
	}
	path := req.GetString("path", "")
	project := req.GetString("project", "")
	maxInitialDeltaStr := req.GetString("max_initial_delta", "")
	maxTotalDeltaStr := req.GetString("max_total_delta", "")

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

	cmpResult := comparison.CompareWithSnapshot(baseResult, currentRes, currentSnap)

	if maxInitialDeltaStr != "" || maxTotalDeltaStr != "" {
		var limits budget.Limits
		if maxInitialDeltaStr != "" {
			limitBytes, err := budget.ParseBytes(maxInitialDeltaStr)
			if err != nil {
				return mcpspec.NewToolResultError(fmt.Sprintf("Invalid max_initial_delta: %v", err)), nil
			}
			limits.MaxInitialDelta = &limitBytes
		}
		if maxTotalDeltaStr != "" {
			limitBytes, err := budget.ParseBytes(maxTotalDeltaStr)
			if err != nil {
				return mcpspec.NewToolResultError(fmt.Sprintf("Invalid max_total_delta: %v", err)), nil
			}
			limits.MaxTotalDelta = &limitBytes
		}

		check := budget.CheckComparison(cmpResult, limits)
		resp := measureResponse{
			Result: cmpResult,
			Budget: &check,
		}

		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
		}

		toolRes := mcpspec.NewToolResultText(string(data))
		toolRes.IsError = !check.Passed
		return toolRes, nil
	}

	data, err := json.MarshalIndent(cmpResult, "", "  ")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
	}

	return mcpspec.NewToolResultText(string(data)), nil
}

func handleWorkspaceSummary(ctx context.Context, req mcpspec.CallToolRequest) (*mcpspec.CallToolResult, error) {
	root := req.GetString("root", "")
	projects, err := parseStringSlice(req.GetArguments(), "projects")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Invalid parameter: %v", err)), nil
	}
	allowNxFallback := req.GetBool("allow_nx_fallback", false)

	wsResult, err := workspace.Analyze(ctx, workspace.AnalyzeOptions{
		Root:            root,
		ExplicitRoot:    root != "",
		Projects:        projects,
		WithCompression: true,
		AllowNxFallback: allowNxFallback,
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
