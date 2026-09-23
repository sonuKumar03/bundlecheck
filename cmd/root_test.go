package cmd

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestExitCodeMatrix(t *testing.T) {
	minimalStats := filepath.Join("..", "testdata", "minimal", "stats.json")

	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{
			name:     "success - valid scan",
			args:     []string{"scan", minimalStats, "--format", "json"},
			wantCode: ExitCodeSuccess, // 0
		},
		{
			name:     "success - gate with generous budget",
			args:     []string{"gate", minimalStats, "--max-initial", "10MB"},
			wantCode: ExitCodeSuccess, // 0
		},
		{
			name:     "policy violation - gate budget breached",
			args:     []string{"gate", minimalStats, "--max-initial", "100B"},
			wantCode: ExitCodePolicyViolation, // 1
		},
		{
			name:     "usage error - unknown flag",
			args:     []string{"scan", minimalStats, "--nonexistent-flag"},
			wantCode: ExitCodeUsage, // 2
		},
		{
			name:     "usage error - unsupported format",
			args:     []string{"scan", minimalStats, "--format", "xml"},
			wantCode: ExitCodeUsage, // 2
		},
		{
			name:     "usage error - missing scan target",
			args:     []string{"scan"},
			wantCode: ExitCodeUsage, // 2
		},
		{
			name:     "usage error - invalid bytes flag",
			args:     []string{"gate", minimalStats, "--max-initial", "invalid-bytes"},
			wantCode: ExitCodeUsage, // 2
		},
		{
			name:     "execution error - nonexistent file",
			args:     []string{"scan", filepath.Join("..", "testdata", "does-not-exist.json")},
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
