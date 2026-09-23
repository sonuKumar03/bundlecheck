package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/sonuKumar03/bundleradar/internal/mcp"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
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
	if !strings.Contains(out, "bundle_scan") {
		t.Errorf("expected help output to mention bundle_scan, got:\n%s", out)
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
	minimalStats, _ := filepath.Abs("../testdata/minimal/stats.json")

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
	if !ok || serverInfo["name"] != "bundleradar" {
		t.Errorf("expected server name bundleradar, got %v", serverInfo)
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
	if len(tools) != 4 {
		t.Errorf("expected 4 tools in tools/list, got %d", len(tools))
	}

	// 3. Send tools/call (bundle_scan)
	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "bundle_scan",
			"arguments": map[string]any{
				"path": minimalStats,
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

	// 4. Send resources/read (bundleradar://rules)
	readReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "resources/read",
		"params": map[string]any{
			"uri": "bundleradar://rules",
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
	minimalStats, _ := filepath.Abs("../testdata/minimal/stats.json")

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
			"name": "bundle_scan",
			"arguments": map[string]any{
				"path": minimalStats,
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
	minimalStats, _ := filepath.Abs("../testdata/minimal/stats.json")

	t.Run("explicit threshold equivalence with gate", func(t *testing.T) {
		// CLI gate with max_initial
		var cliOut, cliErr bytes.Buffer
		cliCode := Execute([]string{"gate", minimalStats, "--max-initial", "10B", "--format", "json"}, &cliOut, &cliErr)
		if cliCode != 1 {
			t.Fatalf("expected CLI exit 1, got %d. stderr: %s", cliCode, cliErr.String())
		}
		var cliResult bundleradar.EvaluationResult
		if err := json.Unmarshal(cliOut.Bytes(), &cliResult); err != nil {
			t.Fatalf("failed to parse CLI gate JSON: %v", err)
		}

		// MCP bundle_gate with max_initial
		mcpReq := map[string]any{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/call",
			"params": map[string]any{
				"name": "bundle_gate",
				"arguments": map[string]any{
					"path":        minimalStats,
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
		var mcpResult bundleradar.EvaluationResult
		if err := json.Unmarshal([]byte(text), &mcpResult); err != nil {
			t.Fatalf("failed to parse MCP gate JSON: %v", err)
		}

		if cliResult.Passed != mcpResult.Passed {
			t.Errorf("passed mismatch: CLI=%v, MCP=%v", cliResult.Passed, mcpResult.Passed)
		}
		if len(cliResult.Violations) != len(mcpResult.Violations) {
			t.Fatalf("violation count mismatch: CLI=%d, MCP=%d", len(cliResult.Violations), len(mcpResult.Violations))
		}
	})
}
