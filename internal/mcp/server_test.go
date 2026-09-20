package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
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

