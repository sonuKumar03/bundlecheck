package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestJSONContractsV1(t *testing.T) {
	minimalDir := filepath.Join("..", "testdata", "minimal")
	contractsDir := filepath.Join("..", "testdata", "contracts", "v1")

	tests := []struct {
		name         string
		args         []string
		expectedFile string
	}{
		{
			name:         "summary contract",
			args:         []string{"summary", minimalDir, "--format", "json"},
			expectedFile: filepath.Join(contractsDir, "summary.json"),
		},
		{
			name:         "compare contract",
			args:         []string{"compare", filepath.Join("..", "testdata", "comparison", "before.json"), filepath.Join("..", "testdata", "comparison", "after.json"), "--format", "json"},
			expectedFile: filepath.Join(contractsDir, "compare.json"),
		},
		{
			name:         "why contract",
			args:         []string{"why", minimalDir, "lodash", "--format", "json"},
			expectedFile: filepath.Join(contractsDir, "why.json"),
		},
		{
			name:         "suggest contract",
			args:         []string{"suggest", minimalDir, "--format", "json"},
			expectedFile: filepath.Join(contractsDir, "suggest.json"),
		},
		{
			name:         "check contract",
			args:         []string{"check", minimalDir, "--max-initial", "2KB", "--format", "json"},
			expectedFile: filepath.Join(contractsDir, "check.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Execute(tt.args, &stdout, &stderr)
			if code != ExitCodeSuccess {
				t.Fatalf("command %v failed with code %d: %s", tt.args, code, stderr.String())
			}

			expectedBytes, err := os.ReadFile(tt.expectedFile)
			if err != nil {
				t.Fatalf("failed to read expected golden file %s: %v", tt.expectedFile, err)
			}

			var gotJSON, wantJSON any
			if err := json.Unmarshal(stdout.Bytes(), &gotJSON); err != nil {
				t.Fatalf("invalid JSON output: %v\nOutput: %s", err, stdout.String())
			}
			if err := json.Unmarshal(expectedBytes, &wantJSON); err != nil {
				t.Fatalf("invalid expected JSON in %s: %v", tt.expectedFile, err)
			}

			if !reflect.DeepEqual(gotJSON, wantJSON) {
				t.Errorf("JSON output does not match golden contract %s.\nGot:\n%s\nWant:\n%s",
					tt.expectedFile, stdout.String(), string(expectedBytes))
			}
		})
	}
}
