package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/sonuKumar03/bundlecheck/internal/budget"
	"github.com/sonuKumar03/bundlecheck/internal/mcp"
)

func TestMCPCommand_Help(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := Execute([]string{"mcp", "--help"}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d. stderr: %s", exitCode, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "Model Context Protocol") {
		t.Errorf("expected help output to mention Model Context Protocol, got:\n%s", out)
	}
	if !strings.Contains(out, "bundle_summary") {
		t.Errorf("expected help output to mention bundle_summary, got:\n%s", out)
	}
}

func TestMCPStdioServer_LegacyProtocol(t *testing.T) {
	s := mcp.NewServer()
	stdioServer := server.NewStdioServer(s)

	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- stdioServer.Listen(ctx, stdinReader, stdoutWriter)
	}()

	decoder := json.NewDecoder(stdoutReader)
	minimalPath, _ := filepath.Abs("../testdata/minimal")

	// 1. Send JSON-RPC initialize
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"clientInfo": map[string]any{
				"name":    "test-runner",
				"version": "1.0",
			},
			"capabilities": map[string]any{},
		},
	}
	initData, _ := json.Marshal(initReq)
	initData = append(initData, '\n')
	if _, err := stdinWriter.Write(initData); err != nil {
		t.Fatalf("failed to write initialize request: %v", err)
	}

	var initResp map[string]any
	if err := decoder.Decode(&initResp); err != nil {
		t.Fatalf("failed to decode initialize response: %v", err)
	}
	if initResp["error"] != nil {
		t.Fatalf("initialize returned error: %v", initResp["error"])
	}
	result, ok := initResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result map in initialize response: %v", initResp)
	}
	serverInfo, ok := result["serverInfo"].(map[string]any)
	if !ok || serverInfo["name"] != "bundlecheck" {
		t.Errorf("expected server name bundlecheck, got %v", serverInfo)
	}

	// 2. Send tools/list
	listReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]any{},
	}
	listData, _ := json.Marshal(listReq)
	listData = append(listData, '\n')
	if _, err := stdinWriter.Write(listData); err != nil {
		t.Fatalf("failed to write tools/list request: %v", err)
	}

	var listResp map[string]any
	if err := decoder.Decode(&listResp); err != nil {
		t.Fatalf("failed to decode tools/list response: %v", err)
	}
	if listResp["error"] != nil {
		t.Fatalf("tools/list returned error: %v", listResp["error"])
	}
	listResult := listResp["result"].(map[string]any)
	tools := listResult["tools"].([]any)
	if len(tools) != 6 {
		t.Errorf("expected 6 tools in tools/list, got %d", len(tools))
	}

	// 3. Send tools/call (bundle_summary)
	callReq := map[string]any{
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
	callData, _ := json.Marshal(callReq)
	callData = append(callData, '\n')
	if _, err := stdinWriter.Write(callData); err != nil {
		t.Fatalf("failed to write tools/call request: %v", err)
	}

	var callResp map[string]any
	if err := decoder.Decode(&callResp); err != nil {
		t.Fatalf("failed to decode tools/call response: %v", err)
	}
	if callResp["error"] != nil {
		t.Fatalf("tools/call returned error: %v", callResp["error"])
	}
	callResult := callResp["result"].(map[string]any)
	content := callResult["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("expected non-empty tool call content")
	}

	// 4. Send resources/read (bundlecheck://rules)
	readReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "resources/read",
		"params": map[string]any{
			"uri": "bundlecheck://rules",
		},
	}
	readData, _ := json.Marshal(readReq)
	readData = append(readData, '\n')
	if _, err := stdinWriter.Write(readData); err != nil {
		t.Fatalf("failed to write resources/read request: %v", err)
	}

	var readResp map[string]any
	if err := decoder.Decode(&readResp); err != nil {
		t.Fatalf("failed to decode resources/read response: %v", err)
	}
	if readResp["error"] != nil {
		t.Fatalf("resources/read returned error: %v", readResp["error"])
	}
	readResult := readResp["result"].(map[string]any)
	contents := readResult["contents"].([]any)
	if len(contents) == 0 {
		t.Fatalf("expected non-empty resource contents")
	}

	// Clean up pipes and context
	_ = stdinWriter.Close()
	_ = stdoutWriter.Close()
	cancel()
}

func TestMCPStdioServer_ModernProtocol_2026_07_28(t *testing.T) {
	s := mcp.NewServer()
	stdioServer := server.NewStdioServer(s)

	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- stdioServer.Listen(ctx, stdinReader, stdoutWriter)
	}()

	decoder := json.NewDecoder(stdoutReader)
	minimalPath, _ := filepath.Abs("../testdata/minimal")

	// 1. Send server/discover with 2026-07-28 protocol metadata
	discoverReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      100,
		"method":  "server/discover",
		"params": map[string]any{
			"_meta": map[string]any{
				"io.modelcontextprotocol/protocolVersion": "2026-07-28",
				"io.modelcontextprotocol/clientInfo": map[string]any{
					"name":    "modern-client",
					"version": "1.0.0",
				},
				"io.modelcontextprotocol/clientCapabilities": map[string]any{},
			},
		},
	}
	discoverData, _ := json.Marshal(discoverReq)
	discoverData = append(discoverData, '\n')
	if _, err := stdinWriter.Write(discoverData); err != nil {
		t.Fatalf("failed to write discover request: %v", err)
	}

	var discoverResp map[string]any
	if err := decoder.Decode(&discoverResp); err != nil {
		t.Fatalf("failed to decode discover response: %v", err)
	}
	if discoverResp["error"] != nil {
		t.Fatalf("server/discover returned error: %v", discoverResp["error"])
	}

	discoverResult, ok := discoverResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object in discover response: %v", discoverResp)
	}
	supportedVersions, ok := discoverResult["supportedVersions"].([]any)
	if !ok {
		t.Fatalf("expected supportedVersions in discover result: %v", discoverResult)
	}
	found2026 := false
	for _, v := range supportedVersions {
		if v == "2026-07-28" {
			found2026 = true
			break
		}
	}
	if !found2026 {
		t.Errorf("expected 2026-07-28 in supportedVersions, got: %v", supportedVersions)
	}

	// 2. Send modern tools/call
	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      101,
		"method":  "tools/call",
		"params": map[string]any{
			"_meta": map[string]any{
				"io.modelcontextprotocol/protocolVersion": "2026-07-28",
				"io.modelcontextprotocol/clientInfo": map[string]any{
					"name":    "modern-client",
					"version": "1.0.0",
				},
				"io.modelcontextprotocol/clientCapabilities": map[string]any{},
			},
			"name": "bundle_summary",
			"arguments": map[string]any{
				"path": minimalPath,
			},
		},
	}
	callData, _ := json.Marshal(callReq)
	callData = append(callData, '\n')
	if _, err := stdinWriter.Write(callData); err != nil {
		t.Fatalf("failed to write modern tools/call request: %v", err)
	}

	var callResp map[string]any
	if err := decoder.Decode(&callResp); err != nil {
		t.Fatalf("failed to decode modern tools/call response: %v", err)
	}
	if callResp["error"] != nil {
		t.Fatalf("modern tools/call returned error: %v", callResp["error"])
	}
	callResult := callResp["result"].(map[string]any)
	content := callResult["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("expected content in modern tools/call response")
	}

	// Clean up
	_ = stdinWriter.Close()
	_ = stdoutWriter.Close()
	cancel()
}

func TestMCPAndCLIEquivalence(t *testing.T) {
	ctx := context.Background()
	s := mcp.NewServer()
	minimalPath, _ := filepath.Abs("../testdata/minimal")
	baselinePath, _ := filepath.Abs("../testdata/comparison/before.json")

	t.Run("config file budget and disallowed package equivalence", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Copy minimal fixture
		statsData, _ := os.ReadFile(filepath.Join(minimalPath, "stats.json"))
		_ = os.WriteFile(filepath.Join(tmpDir, "stats.json"), statsData, 0644)
		browserDir := filepath.Join(tmpDir, "browser")
		_ = os.MkdirAll(browserDir, 0755)
		htmlData, _ := os.ReadFile(filepath.Join(minimalPath, "browser", "index.html"))
		_ = os.WriteFile(filepath.Join(browserDir, "index.html"), htmlData, 0644)
		jsData, _ := os.ReadFile(filepath.Join(minimalPath, "browser", "main.js"))
		_ = os.WriteFile(filepath.Join(browserDir, "main.js"), jsData, 0644)

		cfgContent := []byte("budgets:\n  initial_js_max: 50B\nrules:\n  disallow_packages:\n    - lodash\n")
		cfgPath := filepath.Join(tmpDir, ".bundlecheck.yml")
		_ = os.WriteFile(cfgPath, cfgContent, 0644)

		// 1. Run CLI check
		var cliOut, cliErr bytes.Buffer
		cliCode := Execute([]string{"check", tmpDir, "--config", cfgPath, "--format", "json"}, &cliOut, &cliErr)
		if cliCode != 1 {
			t.Fatalf("expected CLI exit 1, got %d: %s", cliCode, cliErr.String())
		}
		var cliResult budget.CheckResult
		if err := json.Unmarshal(cliOut.Bytes(), &cliResult); err != nil {
			t.Fatalf("failed to unmarshal CLI json: %v; stderr: %s", err, cliErr.String())
		}

		// 2. Run MCP bundle_check
		mcpReq := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params": map[string]any{
				"name": "bundle_check",
				"arguments": map[string]any{
					"path": tmpDir,
				},
			},
		}
		reqJSON, _ := json.Marshal(mcpReq)
		mcpResp := s.HandleMessage(ctx, reqJSON)
		respData, _ := json.Marshal(mcpResp)
		var mcpParsed map[string]any
		_ = json.Unmarshal(respData, &mcpParsed)
		toolRes := mcpParsed["result"].(map[string]any)
		if toolRes["isError"] != true {
			t.Errorf("expected MCP isError: true on failing budget")
		}
		content := toolRes["content"].([]any)
		text := content[0].(map[string]any)["text"].(string)
		var mcpResult budget.CheckResult
		if err := json.Unmarshal([]byte(text), &mcpResult); err != nil {
			t.Fatalf("failed to unmarshal MCP json: %v", err)
		}

		// Verify complete equivalence
		if cliResult.Passed != mcpResult.Passed {
			t.Errorf("expected Passed match: CLI=%v, MCP=%v", cliResult.Passed, mcpResult.Passed)
		}
		if len(cliResult.Violations) != len(mcpResult.Violations) {
			t.Fatalf("expected violation count match: CLI=%d, MCP=%d", len(cliResult.Violations), len(mcpResult.Violations))
		}
		for i := range cliResult.Violations {
			if cliResult.Violations[i].Metric != mcpResult.Violations[i].Metric {
				t.Errorf("violation %d metric mismatch: CLI=%q, MCP=%q", i, cliResult.Violations[i].Metric, mcpResult.Violations[i].Metric)
			}
			if cliResult.Violations[i].Actual != mcpResult.Violations[i].Actual {
				t.Errorf("violation %d actual mismatch: CLI=%d, MCP=%d", i, cliResult.Violations[i].Actual, mcpResult.Violations[i].Actual)
			}
		}
	})

	t.Run("explicit threshold equivalence", func(t *testing.T) {
		// CLI check with max_initial
		var cliOut, cliErr bytes.Buffer
		cliCode := Execute([]string{"check", minimalPath, "--max-initial", "10B", "--format", "json"}, &cliOut, &cliErr)
		if cliCode != 1 {
			t.Fatalf("expected CLI exit 1, got %d", cliCode)
		}
		var cliResult budget.CheckResult
		_ = json.Unmarshal(cliOut.Bytes(), &cliResult)

		// MCP bundle_check with max_initial
		mcpReq := map[string]any{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/call",
			"params": map[string]any{
				"name": "bundle_check",
				"arguments": map[string]any{
					"path":        minimalPath,
					"max_initial": "10B",
				},
			},
		}
		reqJSON, _ := json.Marshal(mcpReq)
		mcpResp := s.HandleMessage(ctx, reqJSON)
		respData, _ := json.Marshal(mcpResp)
		var mcpParsed map[string]any
		_ = json.Unmarshal(respData, &mcpParsed)
		toolRes := mcpParsed["result"].(map[string]any)
		content := toolRes["content"].([]any)
		text := content[0].(map[string]any)["text"].(string)
		var mcpResult budget.CheckResult
		_ = json.Unmarshal([]byte(text), &mcpResult)

		if cliResult.Violations[0].Actual != mcpResult.Violations[0].Actual {
			t.Errorf("actual mismatch: CLI=%d, MCP=%d", cliResult.Violations[0].Actual, mcpResult.Violations[0].Actual)
		}
	})

	t.Run("baseline delta equivalence", func(t *testing.T) {
		// CLI check with baseline and max_initial_delta
		var cliOut, cliErr bytes.Buffer
		cliCode := Execute([]string{"check", minimalPath, "--baseline", baselinePath, "--max-initial-delta", "-1MB", "--format", "json"}, &cliOut, &cliErr)
		if cliCode != 1 {
			t.Fatalf("expected CLI exit 1, got %d", cliCode)
		}
		var cliResult budget.CheckResult
		_ = json.Unmarshal(cliOut.Bytes(), &cliResult)

		// MCP bundle_check with baseline and max_initial_delta
		mcpReq := map[string]any{
			"jsonrpc": "2.0",
			"id":      3,
			"method":  "tools/call",
			"params": map[string]any{
				"name": "bundle_check",
				"arguments": map[string]any{
					"path":              minimalPath,
					"baseline":          baselinePath,
					"max_initial_delta": "-1MB",
				},
			},
		}
		reqJSON, _ := json.Marshal(mcpReq)
		mcpResp := s.HandleMessage(ctx, reqJSON)
		respData, _ := json.Marshal(mcpResp)
		var mcpParsed map[string]any
		_ = json.Unmarshal(respData, &mcpParsed)
		toolRes := mcpParsed["result"].(map[string]any)
		content := toolRes["content"].([]any)
		text := content[0].(map[string]any)["text"].(string)
		var mcpResult budget.CheckResult
		_ = json.Unmarshal([]byte(text), &mcpResult)

		if cliResult.Violations[0].Metric != mcpResult.Violations[0].Metric {
			t.Errorf("metric mismatch: CLI=%q, MCP=%q", cliResult.Violations[0].Metric, mcpResult.Violations[0].Metric)
		}
		if cliResult.Violations[0].Actual != mcpResult.Violations[0].Actual {
			t.Errorf("actual delta mismatch: CLI=%d, MCP=%d", cliResult.Violations[0].Actual, mcpResult.Violations[0].Actual)
		}
	})
}
