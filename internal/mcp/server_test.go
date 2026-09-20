package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpspec "github.com/mark3labs/mcp-go/mcp"

	"bundlecheck/internal/advisor"
	"bundlecheck/internal/analysis"
	"bundlecheck/internal/budget"
	"bundlecheck/internal/comparison"
	"bundlecheck/internal/graph"
	"bundlecheck/internal/workspace"
)

func TestNewServer_Metadata(t *testing.T) {
	s := NewServer()
	if s == nil {
		t.Fatal("expected non-nil server")
	}

	tools := s.ListTools()
	expectedTools := []string{
		"bundle_summary",
		"bundle_why",
		"bundle_suggest",
		"bundle_check",
		"bundle_measure",
		"workspace_summary",
	}

	for _, name := range expectedTools {
		if _, ok := tools[name]; !ok {
			t.Errorf("missing expected tool: %s", name)
		}
	}

	resources := s.ListResources()
	if _, ok := resources["bundlecheck://rules"]; !ok {
		t.Errorf("missing expected resource bundlecheck://rules")
	}
}

func TestHandleSummary(t *testing.T) {
	ctx := context.Background()
	minimalPath, _ := filepath.Abs("../../testdata/minimal")

	t.Run("basic summary", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_summary",
				Arguments: map[string]any{
					"path": minimalPath,
				},
			},
		}

		res, err := handleSummary(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatalf("expected success, got error: %v", res.Content[0])
		}

		textContent, ok := mcpspec.AsTextContent(res.Content[0])
		if !ok {
			t.Fatalf("expected text content")
		}

		var summary analysis.AnalysisResult
		if err := json.Unmarshal([]byte(textContent.Text), &summary); err != nil {
			t.Fatalf("failed to unmarshal JSON summary: %v", err)
		}

		if summary.Summary.InitialJS <= 0 {
			t.Errorf("expected InitialJS > 0, got %d", summary.Summary.InitialJS)
		}
		if len(summary.Packages) == 0 {
			t.Fatalf("expected at least one package, got 0")
		}
		if summary.Packages[0].Name != "lodash" {
			t.Errorf("expected lodash package, got %s", summary.Packages[0].Name)
		}
	})

	t.Run("top parameter restricts packages", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_summary",
				Arguments: map[string]any{
					"path": minimalPath,
					"top":  1,
				},
			},
		}

		res, err := handleSummary(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var summary analysis.AnalysisResult
		_ = json.Unmarshal([]byte(textContent.Text), &summary)

		if len(summary.Packages) > 1 {
			t.Errorf("expected at most 1 package, got %d", len(summary.Packages))
		}
	})

	t.Run("filter parameter", func(t *testing.T) {
		reqMatch := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_summary",
				Arguments: map[string]any{
					"path":   minimalPath,
					"filter": "loda",
				},
			},
		}
		resMatch, _ := handleSummary(ctx, reqMatch)
		textContentMatch, _ := mcpspec.AsTextContent(resMatch.Content[0])
		var summaryMatch analysis.AnalysisResult
		_ = json.Unmarshal([]byte(textContentMatch.Text), &summaryMatch)
		if len(summaryMatch.Packages) != 1 || summaryMatch.Packages[0].Name != "lodash" {
			t.Errorf("expected 1 match for 'loda', got %v", summaryMatch.Packages)
		}

		reqNoMatch := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_summary",
				Arguments: map[string]any{
					"path":   minimalPath,
					"filter": "nonexistent-pkg",
				},
			},
		}
		resNoMatch, _ := handleSummary(ctx, reqNoMatch)
		textContentNoMatch, _ := mcpspec.AsTextContent(resNoMatch.Content[0])
		var summaryNoMatch analysis.AnalysisResult
		_ = json.Unmarshal([]byte(textContentNoMatch.Text), &summaryNoMatch)
		if len(summaryNoMatch.Packages) != 0 {
			t.Errorf("expected 0 matches for 'nonexistent-pkg', got %d", len(summaryNoMatch.Packages))
		}
	})

	t.Run("nonexistent path returns error", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_summary",
				Arguments: map[string]any{
					"path": "/path/does/not/exist",
				},
			},
		}
		res, err := handleSummary(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected error tool result, got success")
		}
	})
}

func TestHandleSummary_NxProject(t *testing.T) {
	ctx := context.Background()
	workspacePath, _ := filepath.Abs("../../testdata/nx-workspace")
	if _, err := os.Stat(filepath.Join(workspacePath, "dist", "apps", "portal", "stats.json")); os.IsNotExist(err) {
		if os.Getenv("BUNDLECHECK_REQUIRE_E2E") != "" {
			t.Fatalf("required E2E artifact missing: %s/dist/apps/portal/stats.json", workspacePath)
		}
		t.Skip("portal build artifacts not present in testdata/nx-workspace")
	}

	t.Run("resolve by project name", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_summary",
				Arguments: map[string]any{
					"path":    workspacePath,
					"project": "portal",
				},
			},
		}

		res, err := handleSummary(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			textContent, _ := mcpspec.AsTextContent(res.Content[0])
			t.Fatalf("expected success, got error: %s", textContent.Text)
		}

		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var summary analysis.AnalysisResult
		if err := json.Unmarshal([]byte(textContent.Text), &summary); err != nil {
			t.Fatalf("failed to unmarshal JSON summary: %v", err)
		}
		if summary.Summary.InitialJS <= 0 {
			t.Errorf("expected portal InitialJS > 0, got %d", summary.Summary.InitialJS)
		}
	})

	t.Run("invalid project name returns error", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_summary",
				Arguments: map[string]any{
					"path":    workspacePath,
					"project": "nonexistent-app",
				},
			},
		}

		res, err := handleSummary(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected error for nonexistent project, got success")
		}
	})
}

func TestHandleSummary_MultiProject(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()

	minStats, err := os.ReadFile("../../testdata/minimal/stats.json")
	if err != nil {
		t.Fatalf("failed to read minimal stats fixture: %v", err)
	}
	minHTML, err := os.ReadFile("../../testdata/minimal/browser/index.html")
	if err != nil {
		t.Fatalf("failed to read minimal index.html fixture: %v", err)
	}
	minJS, err := os.ReadFile("../../testdata/minimal/browser/main.js")
	if err != nil {
		t.Fatalf("failed to read minimal main.js fixture: %v", err)
	}

	for _, app := range []string{"portal", "admin-dashboard"} {
		appDist := filepath.Join(tmp, "dist", "apps", app, "browser")
		if err := os.MkdirAll(appDist, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmp, "dist", "apps", app, "stats.json"), minStats, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(appDist, "index.html"), minHTML, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(appDist, "main.js"), minJS, 0644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("resolve by project name", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_summary",
				Arguments: map[string]any{
					"path":    tmp,
					"project": "portal",
				},
			},
		}

		res, err := handleSummary(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			textContent, _ := mcpspec.AsTextContent(res.Content[0])
			t.Fatalf("expected success, got error: %s", textContent.Text)
		}

		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var summary analysis.AnalysisResult
		if err := json.Unmarshal([]byte(textContent.Text), &summary); err != nil {
			t.Fatalf("failed to unmarshal JSON summary: %v", err)
		}
		if summary.Summary.InitialJS <= 0 {
			t.Errorf("expected portal InitialJS > 0, got %d", summary.Summary.InitialJS)
		}
	})

	t.Run("invalid project name returns error", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_summary",
				Arguments: map[string]any{
					"path":    tmp,
					"project": "nonexistent-app",
				},
			},
		}

		res, err := handleSummary(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected error for nonexistent project, got success")
		}
	})

	t.Run("ambiguous projects without project flag returns error", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_summary",
				Arguments: map[string]any{
					"path": tmp,
				},
			},
		}

		res, err := handleSummary(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected error when multiple projects exist without project flag, got success")
		}
	})
}

func TestHandleWhy(t *testing.T) {
	ctx := context.Background()
	minimalPath, _ := filepath.Abs("../../testdata/minimal")

	t.Run("trace existing package", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_why",
				Arguments: map[string]any{
					"path":    minimalPath,
					"package": "lodash",
				},
			},
		}

		res, err := handleWhy(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatalf("expected success, got error: %v", res.Content[0])
		}

		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var why graph.WhyResult
		if err := json.Unmarshal([]byte(textContent.Text), &why); err != nil {
			t.Fatalf("failed to unmarshal why result: %v", err)
		}

		if !why.Found {
			t.Errorf("expected lodash to be found")
		}
		if len(why.Chains) == 0 {
			t.Errorf("expected at least one chain for lodash")
		}
	})

	t.Run("trace nonexistent package", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_why",
				Arguments: map[string]any{
					"path":    minimalPath,
					"package": "nonexistent-lib",
				},
			},
		}

		res, err := handleWhy(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatalf("expected success with Found=false, got tool error: %v", res.Content[0])
		}

		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var why graph.WhyResult
		_ = json.Unmarshal([]byte(textContent.Text), &why)

		if why.Found {
			t.Errorf("expected nonexistent-lib to not be found")
		}
	})

	t.Run("missing package argument returns error", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name:      "bundle_why",
				Arguments: map[string]any{},
			},
		}

		res, err := handleWhy(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected error result for missing package argument")
		}
	})
}

func TestHandleSuggest(t *testing.T) {
	ctx := context.Background()
	lazyPath, _ := filepath.Abs("../../testdata/lazy-import")

	t.Run("generates optimization suggestions", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_suggest",
				Arguments: map[string]any{
					"path":        lazyPath,
					"min_savings": 100,
				},
			},
		}

		res, err := handleSuggest(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatalf("expected success, got error: %v", res.Content[0])
		}

		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var advRes advisor.AdvisorResult
		if err := json.Unmarshal([]byte(textContent.Text), &advRes); err != nil {
			t.Fatalf("failed to unmarshal suggestions: %v", err)
		}

		if len(advRes.Suggestions) == 0 {
			t.Fatalf("expected suggestions for lazy-import fixture")
		}

		hasPdfOrLarge := false
		for _, s := range advRes.Suggestions {
			if s.Target == "pdfjs-dist" || s.Savings > 0 {
				hasPdfOrLarge = true
				break
			}
		}
		if !hasPdfOrLarge {
			t.Errorf("expected advice for pdfjs-dist or high-impact savings")
		}
	})

	t.Run("min_savings filters small opportunities", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_suggest",
				Arguments: map[string]any{
					"path":        lazyPath,
					"min_savings": 100000000, // Very high threshold
				},
			},
		}

		res, _ := handleSuggest(ctx, req)
		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var advRes advisor.AdvisorResult
		_ = json.Unmarshal([]byte(textContent.Text), &advRes)

		if len(advRes.Suggestions) != 0 {
			t.Errorf("expected 0 suggestions with huge min_savings, got %d", len(advRes.Suggestions))
		}
	})
}

func TestHandleCheck(t *testing.T) {
	ctx := context.Background()
	minimalPath, _ := filepath.Abs("../../testdata/minimal")

	t.Run("passing budget", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_check",
				Arguments: map[string]any{
					"path":        minimalPath,
					"max_initial": "100KB",
					"max_total":   "500KB",
				},
			},
		}

		res, err := handleCheck(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatalf("expected success, got error: %v", res.Content[0])
		}

		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var checkRes budget.CheckResult
		if err := json.Unmarshal([]byte(textContent.Text), &checkRes); err != nil {
			t.Fatalf("failed to unmarshal check result: %v", err)
		}

		if !checkRes.Passed {
			t.Errorf("expected budget check to pass, got violations: %v", checkRes.Violations)
		}
		if len(checkRes.Violations) != 0 {
			t.Errorf("expected 0 violations, got %d", len(checkRes.Violations))
		}
	})

	t.Run("failing size budget", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_check",
				Arguments: map[string]any{
					"path":        minimalPath,
					"max_initial": "10B",
				},
			},
		}

		res, err := handleCheck(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var checkRes budget.CheckResult
		_ = json.Unmarshal([]byte(textContent.Text), &checkRes)

		if checkRes.Passed {
			t.Errorf("expected budget check to fail")
		}
		if len(checkRes.Violations) == 0 {
			t.Fatalf("expected at least 1 violation")
		}
		if checkRes.Violations[0].Metric != "initialJs" {
			t.Errorf("expected violation metric initialJs, got %s", checkRes.Violations[0].Metric)
		}
	})

	t.Run("disallowed package rule", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_check",
				Arguments: map[string]any{
					"path":                minimalPath,
					"disallowed_packages": []string{"lodash"},
				},
			},
		}

		res, err := handleCheck(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var checkRes budget.CheckResult
		_ = json.Unmarshal([]byte(textContent.Text), &checkRes)

		if checkRes.Passed {
			t.Errorf("expected check to fail on disallowed package lodash")
		}
		if len(checkRes.Violations) == 0 {
			t.Fatalf("expected violation for disallowed package")
		}
	})

	t.Run("invalid byte format returns error", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_check",
				Arguments: map[string]any{
					"path":        minimalPath,
					"max_initial": "invalid-bytes",
				},
			},
		}

		res, err := handleCheck(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected error result on invalid byte format")
		}
	})
}

func TestHandleMeasure(t *testing.T) {
	ctx := context.Background()
	minimalPath, _ := filepath.Abs("../../testdata/minimal")
	baselinePath, _ := filepath.Abs("../../testdata/comparison/before.json")

	t.Run("compares against baseline", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_measure",
				Arguments: map[string]any{
					"baseline": baselinePath,
					"path":     minimalPath,
				},
			},
		}

		res, err := handleMeasure(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatalf("expected success, got error: %v", res.Content[0])
		}

		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var cmp comparison.Result
		if err := json.Unmarshal([]byte(textContent.Text), &cmp); err != nil {
			t.Fatalf("failed to unmarshal comparison result: %v", err)
		}

		if cmp.Summary.After.InitialJS <= 0 {
			t.Errorf("expected After.InitialJS > 0, got %d", cmp.Summary.After.InitialJS)
		}
	})

	t.Run("regression threshold enforcement", func(t *testing.T) {
		// Passing delta
		reqPass := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_measure",
				Arguments: map[string]any{
					"baseline":          baselinePath,
					"path":              minimalPath,
					"max_initial_delta": "500KB",
				},
			},
		}
		resPass, err := handleMeasure(ctx, reqPass)
		if err != nil || resPass.IsError {
			t.Fatalf("expected pass for 500KB delta, got error: %v", resPass)
		}

		// Failing delta (negative limit: no growth allowed)
		reqFail := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_measure",
				Arguments: map[string]any{
					"baseline":          baselinePath,
					"path":              minimalPath,
					"max_initial_delta": "-1000000B", // Requires shrinking by 1MB
				},
			},
		}
		resFail, err := handleMeasure(ctx, reqFail)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resFail.IsError {
			t.Errorf("expected regression limit breach to return error tool result")
		}
	})

	t.Run("missing baseline argument", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_measure",
				Arguments: map[string]any{
					"path": minimalPath,
				},
			},
		}
		res, _ := handleMeasure(ctx, req)
		if !res.IsError {
			t.Errorf("expected error for missing baseline")
		}
	})
}

func TestHandleWorkspaceSummary(t *testing.T) {
	ctx := context.Background()
	wsPath, _ := filepath.Abs("../../testdata/nx-workspace")

	t.Run("workspace analysis across all apps", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "workspace_summary",
				Arguments: map[string]any{
					"root": wsPath,
				},
			},
		}

		res, err := handleWorkspaceSummary(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatalf("expected success, got error: %v", res.Content[0])
		}

		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var wsRes workspace.Result
		if err := json.Unmarshal([]byte(textContent.Text), &wsRes); err != nil {
			t.Fatalf("failed to unmarshal workspace result: %v", err)
		}

		if len(wsRes.Apps) < 2 {
			t.Errorf("expected at least 2 apps in nx-workspace, got %d", len(wsRes.Apps))
		}

		appNames := map[string]bool{}
		for _, app := range wsRes.Apps {
			appNames[app.Name] = true
		}
		if !appNames["portal"] || !appNames["admin-dashboard"] {
			t.Errorf("expected portal and admin-dashboard, got %v", appNames)
		}
	})

	t.Run("workspace analysis filtered by project", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "workspace_summary",
				Arguments: map[string]any{
					"root":     wsPath,
					"projects": []string{"portal"},
				},
			},
		}

		res, err := handleWorkspaceSummary(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatalf("expected success, got error: %v", res.Content[0])
		}

		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var wsRes workspace.Result
		_ = json.Unmarshal([]byte(textContent.Text), &wsRes)

		if len(wsRes.Apps) != 1 || wsRes.Apps[0].Name != "portal" {
			t.Errorf("expected only portal app, got %v", wsRes.Apps)
		}
	})
}

func TestResource_Rules(t *testing.T) {
	s := NewServer()
	resMap := s.ListResources()
	ruleResource, ok := resMap["bundlecheck://rules"]
	if !ok {
		t.Fatalf("expected resource bundlecheck://rules")
	}

	contents, err := ruleResource.Handler(context.Background(), mcpspec.ReadResourceRequest{
		Params: mcpspec.ReadResourceParams{
			URI: "bundlecheck://rules",
		},
	})
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if len(contents) == 0 {
		t.Fatalf("expected at least 1 resource content")
	}

	textRes, ok := contents[0].(mcpspec.TextResourceContents)
	if !ok {
		t.Fatalf("expected TextResourceContents")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(textRes.Text), &parsed); err != nil {
		t.Fatalf("failed to parse resource JSON: %v", err)
	}

	if _, ok := parsed["recommendations"]; !ok {
		t.Errorf("expected recommendations key in rules")
	}
	if _, ok := parsed["defaultBudgets"]; !ok {
		t.Errorf("expected defaultBudgets key in rules")
	}
}

func TestJSONRPC_Protocol(t *testing.T) {
	ctx := context.Background()
	s := NewServer()
	minimalPath, _ := filepath.Abs("../../testdata/minimal")

	t.Run("initialize protocol", func(t *testing.T) {
		reqJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "initialize",
			"params": {
				"protocolVersion": "2024-11-05",
				"clientInfo": {
					"name": "test-client",
					"version": "1.0.0"
				},
				"capabilities": {}
			}
		}`)

		resp := s.HandleMessage(ctx, reqJSON)
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("failed to marshal response: %v", err)
		}

		var parsed map[string]any
		_ = json.Unmarshal(data, &parsed)
		if parsed["error"] != nil {
			t.Fatalf("initialize returned error: %v", parsed["error"])
		}
		result, ok := parsed["result"].(map[string]any)
		if !ok {
			t.Fatalf("expected result object")
		}
		serverInfo := result["serverInfo"].(map[string]any)
		if serverInfo["name"] != "bundlecheck" {
			t.Errorf("expected server name bundlecheck, got %v", serverInfo["name"])
		}
	})

	t.Run("tools/list protocol", func(t *testing.T) {
		reqJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 2,
			"method": "tools/list",
			"params": {}
		}`)

		resp := s.HandleMessage(ctx, reqJSON)
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("failed to marshal response: %v", err)
		}

		var parsed map[string]any
		_ = json.Unmarshal(data, &parsed)
		if parsed["error"] != nil {
			t.Fatalf("tools/list returned error: %v", parsed["error"])
		}

		result := parsed["result"].(map[string]any)
		toolsList := result["tools"].([]any)
		if len(toolsList) != 6 {
			t.Errorf("expected 6 tools, got %d", len(toolsList))
		}
	})

	t.Run("tools/call protocol", func(t *testing.T) {
		reqPayload := map[string]any{
			"jsonrpc": "2.0",
			"id":      3,
			"method":  "tools/call",
			"params": map[string]any{
				"name": "bundle_summary",
				"arguments": map[string]any{
					"path": minimalPath,
				},
			},
		}
		reqJSON, _ := json.Marshal(reqPayload)

		resp := s.HandleMessage(ctx, reqJSON)
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("failed to marshal response: %v", err)
		}

		var parsed map[string]any
		_ = json.Unmarshal(data, &parsed)
		if parsed["error"] != nil {
			t.Fatalf("tools/call returned error: %v", parsed["error"])
		}

		result := parsed["result"].(map[string]any)
		content := result["content"].([]any)
		if len(content) == 0 {
			t.Fatalf("expected non-empty content")
		}
		firstItem := content[0].(map[string]any)
		text := firstItem["text"].(string)
		if text == "" {
			t.Errorf("expected non-empty text in result")
		}
	})

	t.Run("resources/read protocol", func(t *testing.T) {
		reqPayload := map[string]any{
			"jsonrpc": "2.0",
			"id":      4,
			"method":  "resources/read",
			"params": map[string]any{
				"uri": "bundlecheck://rules",
			},
		}
		reqJSON, _ := json.Marshal(reqPayload)

		resp := s.HandleMessage(ctx, reqJSON)
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("failed to marshal response: %v", err)
		}

		var parsed map[string]any
		_ = json.Unmarshal(data, &parsed)
		if parsed["error"] != nil {
			t.Fatalf("resources/read returned error: %v", parsed["error"])
		}

		result := parsed["result"].(map[string]any)
		contents := result["contents"].([]any)
		if len(contents) == 0 {
			t.Fatalf("expected non-empty contents")
		}
	})
}

func TestHandleSummary_DirectStatsFile(t *testing.T) {
	ctx := context.Background()
	statsFilePath, _ := filepath.Abs("../../testdata/minimal/stats.json")

	req := mcpspec.CallToolRequest{
		Params: mcpspec.CallToolParams{
			Name: "bundle_summary",
			Arguments: map[string]any{
				"path": statsFilePath,
			},
		},
	}

	res, err := handleSummary(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success with direct stats file path, got error: %v", res.Content[0])
	}
}

func TestHandleCheck_MaxTotalViolation(t *testing.T) {
	ctx := context.Background()
	minimalPath, _ := filepath.Abs("../../testdata/minimal")

	req := mcpspec.CallToolRequest{
		Params: mcpspec.CallToolParams{
			Name: "bundle_check",
			Arguments: map[string]any{
				"path":      minimalPath,
				"max_total": "100B", // smaller than minimal bundle total
			},
		},
	}

	res, err := handleCheck(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	textContent, _ := mcpspec.AsTextContent(res.Content[0])
	var checkRes budget.CheckResult
	_ = json.Unmarshal([]byte(textContent.Text), &checkRes)

	if checkRes.Passed {
		t.Errorf("expected check to fail for max_total")
	}
	if len(checkRes.Violations) == 0 || checkRes.Violations[0].Metric != "totalJs" {
		t.Errorf("expected violation on totalJs metric, got %v", checkRes.Violations)
	}
}

func TestHandleWorkspaceSummary_InvalidRoot(t *testing.T) {
	ctx := context.Background()
	req := mcpspec.CallToolRequest{
		Params: mcpspec.CallToolParams{
			Name: "workspace_summary",
			Arguments: map[string]any{
				"root": "/tmp/nonexistent-workspace-directory",
			},
		},
	}

	res, err := handleWorkspaceSummary(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected error tool result for invalid workspace root")
	}
}

func TestToolSchemas_ArrayProperties(t *testing.T) {
	ctx := context.Background()
	s := NewServer()

	reqJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 10,
		"method": "tools/list",
		"params": {}
	}`)
	resp := s.HandleMessage(ctx, reqJSON)
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed map[string]any
	_ = json.Unmarshal(data, &parsed)
	result := parsed["result"].(map[string]any)
	toolsList := result["tools"].([]any)

	toolMap := make(map[string]map[string]any)
	for _, raw := range toolsList {
		tObj := raw.(map[string]any)
		toolMap[tObj["name"].(string)] = tObj
	}

	// 1. Check bundle_check.disallowed_packages
	checkTool, ok := toolMap["bundle_check"]
	if !ok {
		t.Fatal("bundle_check tool missing")
	}
	checkSchema := checkTool["inputSchema"].(map[string]any)
	checkProps := checkSchema["properties"].(map[string]any)
	disallowedProp, ok := checkProps["disallowed_packages"].(map[string]any)
	if !ok {
		t.Fatal("disallowed_packages property missing in bundle_check schema")
	}
	if disallowedProp["type"] != "array" {
		t.Errorf("expected disallowed_packages type 'array', got %v", disallowedProp["type"])
	}
	items, ok := disallowedProp["items"].(map[string]any)
	if !ok || items["type"] != "string" {
		t.Errorf("expected items.type 'string' for disallowed_packages, got %v", disallowedProp["items"])
	}

	// 2. Check workspace_summary.projects
	wsTool, ok := toolMap["workspace_summary"]
	if !ok {
		t.Fatal("workspace_summary tool missing")
	}
	wsSchema := wsTool["inputSchema"].(map[string]any)
	wsProps := wsSchema["properties"].(map[string]any)
	projectsProp, ok := wsProps["projects"].(map[string]any)
	if !ok {
		t.Fatal("projects property missing in workspace_summary schema")
	}
	if projectsProp["type"] != "array" {
		t.Errorf("expected projects type 'array', got %v", projectsProp["type"])
	}
	wsItems, ok := projectsProp["items"].(map[string]any)
	if !ok || wsItems["type"] != "string" {
		t.Errorf("expected items.type 'string' for projects, got %v", projectsProp["items"])
	}
}

func TestHandleCheck_InvalidArrayElements(t *testing.T) {
	ctx := context.Background()
	minimalPath, _ := filepath.Abs("../../testdata/minimal")

	t.Run("rejects non-string array elements", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_check",
				Arguments: map[string]any{
					"path":                minimalPath,
					"disallowed_packages": []any{"lodash", 123},
				},
			},
		}

		res, err := handleCheck(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Fatalf("expected error result for non-string array element")
		}
		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		if !strings.Contains(textContent.Text, "element 1 in disallowed_packages must be a string") {
			t.Errorf("expected element type error, got: %s", textContent.Text)
		}
	})

	t.Run("rejects non-array value for disallowed_packages", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_check",
				Arguments: map[string]any{
					"path":                minimalPath,
					"disallowed_packages": "lodash",
				},
			},
		}

		res, err := handleCheck(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Fatalf("expected error result for non-array value")
		}
		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		if !strings.Contains(textContent.Text, "must be an array of strings") {
			t.Errorf("expected array requirement error, got: %s", textContent.Text)
		}
	})
}

func TestHandleWorkspaceSummary_InvalidArrayElements(t *testing.T) {
	ctx := context.Background()

	t.Run("rejects non-string projects elements", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "workspace_summary",
				Arguments: map[string]any{
					"projects": []any{"portal", 42},
				},
			},
		}

		res, err := handleWorkspaceSummary(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Fatalf("expected error result for non-string project element")
		}
		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		if !strings.Contains(textContent.Text, "element 1 in projects must be a string") {
			t.Errorf("expected element type error, got: %s", textContent.Text)
		}
	})

	t.Run("rejects non-array value for projects", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "workspace_summary",
				Arguments: map[string]any{
					"projects": true,
				},
			},
		}

		res, err := handleWorkspaceSummary(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Fatalf("expected error result for non-array value")
		}
		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		if !strings.Contains(textContent.Text, "must be an array of strings") {
			t.Errorf("expected array requirement error, got: %s", textContent.Text)
		}
	})
}

func TestHandleCheck_DisallowedPackages_LazyOnly(t *testing.T) {
	ctx := context.Background()
	lazyFixturePath, _ := filepath.Abs("../../testdata/lazy-import")

	req := mcpspec.CallToolRequest{
		Params: mcpspec.CallToolParams{
			Name: "bundle_check",
			Arguments: map[string]any{
				"path":                lazyFixturePath,
				"disallowed_packages": []string{"date-fns"},
			},
		},
	}

	res, err := handleCheck(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected check to fail when disallowed package is present in lazy bundle")
	}

	textContent, _ := mcpspec.AsTextContent(res.Content[0])
	var checkRes budget.CheckResult
	if err := json.Unmarshal([]byte(textContent.Text), &checkRes); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if checkRes.Passed {
		t.Errorf("expected checkRes.Passed to be false")
	}
	found := false
	for _, v := range checkRes.Violations {
		if strings.Contains(v.Metric, "date-fns") {
			found = true
			if v.Actual != 92160 {
				t.Errorf("expected actual bytes 92160, got %d", v.Actual)
			}
		}
	}
	if !found {
		t.Errorf("expected violation for date-fns in lazy bundle, got: %v", checkRes.Violations)
	}
}

func TestToolsList_Annotations(t *testing.T) {
	ctx := context.Background()
	s := NewServer()

	reqJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 11,
		"method": "tools/list",
		"params": {}
	}`)
	resp := s.HandleMessage(ctx, reqJSON)
	data, _ := json.Marshal(resp)

	var parsed map[string]any
	_ = json.Unmarshal(data, &parsed)
	result := parsed["result"].(map[string]any)
	toolsList := result["tools"].([]any)

	type expectedHints struct {
		readOnly    bool
		destructive bool
		idempotent  bool
		openWorld   bool
	}

	expectations := map[string]expectedHints{
		"bundle_summary":    {readOnly: true, destructive: false, idempotent: true, openWorld: false},
		"bundle_why":        {readOnly: true, destructive: false, idempotent: true, openWorld: false},
		"bundle_suggest":    {readOnly: true, destructive: false, idempotent: true, openWorld: false},
		"bundle_check":      {readOnly: true, destructive: false, idempotent: true, openWorld: false},
		"bundle_measure":    {readOnly: true, destructive: false, idempotent: true, openWorld: false},
		"workspace_summary": {readOnly: false, destructive: false, idempotent: true, openWorld: true},
	}

	for _, raw := range toolsList {
		tool := raw.(map[string]any)
		name := tool["name"].(string)
		exp, ok := expectations[name]
		if !ok {
			t.Errorf("unexpected tool name: %s", name)
			continue
		}

		annotations, ok := tool["annotations"].(map[string]any)
		if !ok {
			t.Fatalf("tool %s missing annotations", name)
		}

		if readOnly, _ := annotations["readOnlyHint"].(bool); readOnly != exp.readOnly {
			t.Errorf("tool %s: expected readOnlyHint %v, got %v", name, exp.readOnly, readOnly)
		}
		if destructive, _ := annotations["destructiveHint"].(bool); destructive != exp.destructive {
			t.Errorf("tool %s: expected destructiveHint %v, got %v", name, exp.destructive, destructive)
		}
		if idempotent, _ := annotations["idempotentHint"].(bool); idempotent != exp.idempotent {
			t.Errorf("tool %s: expected idempotentHint %v, got %v", name, exp.idempotent, idempotent)
		}
		if openWorld, _ := annotations["openWorldHint"].(bool); openWorld != exp.openWorld {
			t.Errorf("tool %s: expected openWorldHint %v, got %v", name, exp.openWorld, openWorld)
		}
	}
}

func TestHandleWorkspaceSummary_ExecutionBoundary(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()

	// Create workspace with nx.json but no projects to cause static parsing failure
	nxJSON := []byte(`{"installation": {"version": "19.0.0"}}`)
	if err := os.WriteFile(filepath.Join(tmp, "nx.json"), nxJSON, 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("fails without fallback when allow_nx_fallback is false", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "workspace_summary",
				Arguments: map[string]any{
					"root": tmp,
				},
			},
		}

		res, err := handleWorkspaceSummary(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Fatalf("expected error when static parsing fails and fallback is disallowed")
		}
		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		if !strings.Contains(textContent.Text, "allow_nx_fallback") {
			t.Errorf("expected error to mention allow_nx_fallback, got: %s", textContent.Text)
		}
	})

	t.Run("attempts fallback when allow_nx_fallback is true", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "workspace_summary",
				Arguments: map[string]any{
					"root":              tmp,
					"allow_nx_fallback": true,
				},
			},
		}

		res, err := handleWorkspaceSummary(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Fatalf("expected error because node_modules/nx is absent")
		}
		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		if !strings.Contains(textContent.Text, "workspace-installed Nx not found") {
			t.Errorf("expected Nx CLI attempt error, got: %s", textContent.Text)
		}
	})
}

func TestFailedBudgetSemantics_DataPreservation(t *testing.T) {
	ctx := context.Background()
	minimalPath, _ := filepath.Abs("../../testdata/minimal")
	baselinePath, _ := filepath.Abs("../../testdata/comparison/before.json")

	t.Run("bundle_check preserves summary and violations on failure", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_check",
				Arguments: map[string]any{
					"path":        minimalPath,
					"max_initial": "10B",
				},
			},
		}

		res, err := handleCheck(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected isError to be true on failed budget")
		}

		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var checkRes budget.CheckResult
		if err := json.Unmarshal([]byte(textContent.Text), &checkRes); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if checkRes.Passed {
			t.Errorf("expected Passed == false")
		}
		if len(checkRes.Violations) == 0 {
			t.Errorf("expected violations, got 0")
		}
		if checkRes.Summary == nil || checkRes.Summary.InitialJS <= 0 {
			t.Errorf("expected non-nil summary with InitialJS > 0, got %v", checkRes.Summary)
		}
	})

	t.Run("bundle_check preserves comparison and violations on baseline delta failure", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_check",
				Arguments: map[string]any{
					"path":              minimalPath,
					"baseline":          baselinePath,
					"max_initial_delta": "-1MB",
				},
			},
		}

		res, err := handleCheck(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected isError to be true on failed baseline delta")
		}

		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var checkRes budget.CheckResult
		if err := json.Unmarshal([]byte(textContent.Text), &checkRes); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if checkRes.Passed {
			t.Errorf("expected Passed == false")
		}
		if len(checkRes.Violations) == 0 {
			t.Errorf("expected violations, got 0")
		}
		if checkRes.Comparison == nil || checkRes.Comparison.Summary.Before.InitialJS <= 0 {
			t.Errorf("expected non-nil comparison with valid data, got %v", checkRes.Comparison)
		}
	})

	t.Run("bundle_measure preserves full comparison and budget violations on regression failure", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_measure",
				Arguments: map[string]any{
					"baseline":          baselinePath,
					"path":              minimalPath,
					"max_initial_delta": "-1MB",
					"max_total_delta":   "-1MB",
				},
			},
		}

		res, err := handleMeasure(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected isError to be true on breached delta threshold")
		}

		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var resp measureResponse
		if err := json.Unmarshal([]byte(textContent.Text), &resp); err != nil {
			t.Fatalf("failed to unmarshal measureResponse JSON: %v", err)
		}

		// Verify comparison data is complete
		if resp.Summary.Before.InitialJS <= 0 || resp.Summary.After.InitialJS <= 0 {
			t.Errorf("expected complete before/after summary, got %v", resp.Summary)
		}
		if len(resp.Packages) == 0 {
			t.Errorf("expected packages list in comparison result, got 0")
		}

		// Verify budget violations are complete
		if resp.Budget == nil {
			t.Fatalf("expected non-nil budget in measure response")
		}
		if resp.Budget.Passed {
			t.Errorf("expected budget.Passed == false")
		}
		if len(resp.Budget.Violations) != 2 {
			t.Errorf("expected 2 violations (initial and total delta), got %d: %v", len(resp.Budget.Violations), resp.Budget.Violations)
		}
	})
}

func TestHandleCheck_FullParityAndConfigFile(t *testing.T) {
	ctx := context.Background()
	minimalPath, _ := filepath.Abs("../../testdata/minimal")
	baselinePath, _ := filepath.Abs("../../testdata/comparison/before.json")

	t.Run("loads config file automatically when present in directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Copy minimal fixture stats.json and browser dir
		statsData, err := os.ReadFile(filepath.Join(minimalPath, "stats.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "stats.json"), statsData, 0644); err != nil {
			t.Fatal(err)
		}
		browserDir := filepath.Join(tmpDir, "browser")
		if err := os.MkdirAll(browserDir, 0755); err != nil {
			t.Fatal(err)
		}
		htmlData, err := os.ReadFile(filepath.Join(minimalPath, "browser", "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(browserDir, "index.html"), htmlData, 0644); err != nil {
			t.Fatal(err)
		}
		jsData, err := os.ReadFile(filepath.Join(minimalPath, "browser", "main.js"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(browserDir, "main.js"), jsData, 0644); err != nil {
			t.Fatal(err)
		}

		// Create .bundlecheck.yml with budget that fails
		cfgContent := []byte("budgets:\n  initial_js_max: 10B\nrules:\n  disallow_packages:\n    - lodash\n")
		if err := os.WriteFile(filepath.Join(tmpDir, ".bundlecheck.yml"), cfgContent, 0644); err != nil {
			t.Fatal(err)
		}

		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_check",
				Arguments: map[string]any{
					"path": tmpDir,
				},
			},
		}

		res, err := handleCheck(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected error tool result from .bundlecheck.yml budget violation")
		}

		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		var checkRes budget.CheckResult
		_ = json.Unmarshal([]byte(textContent.Text), &checkRes)

		if checkRes.Passed {
			t.Errorf("expected checkRes.Passed to be false")
		}
		// Expect both initial JS budget violation and disallowed package lodash violation
		if len(checkRes.Violations) < 2 {
			t.Errorf("expected at least 2 violations from .bundlecheck.yml, got %d: %v", len(checkRes.Violations), checkRes.Violations)
		}
	})

	t.Run("explicit config file parameter", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgFile := filepath.Join(tmpDir, "custom.yml")
		cfgContent := []byte("budgets:\n  initial_js_max: 10B\n")
		if err := os.WriteFile(cfgFile, cfgContent, 0644); err != nil {
			t.Fatal(err)
		}

		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_check",
				Arguments: map[string]any{
					"path":   minimalPath,
					"config": cfgFile,
				},
			},
		}

		res, err := handleCheck(ctx, req)
		if err != nil || !res.IsError {
			t.Fatalf("expected budget check failure from explicit config file")
		}
	})

	t.Run("max_lazy budget threshold", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_check",
				Arguments: map[string]any{
					"path":     minimalPath,
					"max_lazy": "0B",
				},
			},
		}

		res, err := handleCheck(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		// In minimal fixture, LazyJS is 0B, so max_lazy 0B passes
		if res.IsError {
			t.Errorf("expected max_lazy 0B to pass for 0 lazy bytes")
		}
	})

	t.Run("max_total_delta budget threshold", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_check",
				Arguments: map[string]any{
					"path":            minimalPath,
					"baseline":        baselinePath,
					"max_total_delta": "-1MB",
				},
			},
		}

		res, err := handleCheck(ctx, req)
		if err != nil || !res.IsError {
			t.Fatalf("expected error tool result on max_total_delta breach")
		}
	})

	t.Run("requires at least one budget threshold or rule when no config exists", func(t *testing.T) {
		req := mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{
				Name: "bundle_check",
				Arguments: map[string]any{
					"path": minimalPath,
				},
			},
		}

		res, err := handleCheck(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected error when no threshold or rule is provided")
		}
		textContent, _ := mcpspec.AsTextContent(res.Content[0])
		if !strings.Contains(textContent.Text, "at least one budget threshold must be specified") {
			t.Errorf("expected error about budget threshold, got: %s", textContent.Text)
		}
	})
}

