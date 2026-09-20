package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"bundlecheck/internal/mcp"
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

func TestMCPStdioServer_Lifecycle(t *testing.T) {
	s := mcp.NewServer()
	stdioServer := server.NewStdioServer(s)

	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- stdioServer.Listen(ctx, stdinReader, stdoutWriter)
	}()

	// Send JSON-RPC initialize
	req := map[string]any{
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
	reqData, _ := json.Marshal(req)
	reqData = append(reqData, '\n')

	if _, err := stdinWriter.Write(reqData); err != nil {
		t.Fatalf("failed to write request: %v", err)
	}

	decoder := json.NewDecoder(stdoutReader)
	var resp map[string]any
	if err := decoder.Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["error"] != nil {
		t.Fatalf("server returned error: %v", resp["error"])
	}

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result map in response: %v", resp)
	}

	serverInfo, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("expected serverInfo map: %v", result)
	}

	if serverInfo["name"] != "bundlecheck" {
		t.Errorf("expected server name bundlecheck, got %v", serverInfo["name"])
	}

	// Clean up pipes and context
	_ = stdinWriter.Close()
	_ = stdoutWriter.Close()
	cancel()
}
