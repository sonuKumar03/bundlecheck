package cmd

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestExitCodeMatrix(t *testing.T) {
	minimalDir := filepath.Join("..", "testdata", "minimal")

	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{
			name:     "success - valid summary",
			args:     []string{"summary", minimalDir, "--format", "json"},
			wantCode: ExitCodeSuccess, // 0
		},
		{
			name:     "success - check with generous budget",
			args:     []string{"check", minimalDir, "--max-initial", "10MB"},
			wantCode: ExitCodeSuccess, // 0
		},
		{
			name:     "policy violation - check budget breached",
			args:     []string{"check", minimalDir, "--max-initial", "100B"},
			wantCode: ExitCodePolicyViolation, // 1
		},
		{
			name:     "usage error - unknown flag",
			args:     []string{"summary", minimalDir, "--nonexistent-flag"},
			wantCode: ExitCodeUsage, // 2
		},
		{
			name:     "usage error - unsupported format",
			args:     []string{"summary", minimalDir, "--format", "xml"},
			wantCode: ExitCodeUsage, // 2
		},
		{
			name:     "usage error - missing why target",
			args:     []string{"why"},
			wantCode: ExitCodeUsage, // 2
		},
		{
			name:     "usage error - invalid bytes flag",
			args:     []string{"check", minimalDir, "--max-initial", "invalid-bytes"},
			wantCode: ExitCodeUsage, // 2
		},
		{
			name:     "execution error - nonexistent directory",
			args:     []string{"summary", filepath.Join("..", "testdata", "does-not-exist")},
			wantCode: ExitCodeExecution, // 3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Execute(tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("Execute(%v) = %d; want exit code %d (stdout: %s, stderr: %s)",
					tt.args, code, tt.wantCode, stdout.String(), stderr.String())
			}
		})
	}
}
