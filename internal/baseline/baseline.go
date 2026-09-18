// Package baseline manages saving and loading baseline summary metrics.
package baseline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/comparison"
)

const DefaultBaselineFilename = ".bundlecheck/baseline.json"

// ResolvePath returns the provided path or the default baseline location.
func ResolvePath(customPath string) string {
	if customPath != "" {
		return customPath
	}
	return DefaultBaselineFilename
}

// Load reads and parses a saved baseline summary JSON document.
func Load(path string) (*analysis.AnalysisResult, error) {
	resolved := ResolvePath(path)
	if _, err := os.Stat(resolved); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("baseline file %q not found\nHint: run 'bundlecheck baseline' to capture the initial baseline first", resolved)
		}
		return nil, fmt.Errorf("read baseline file %q: %w", resolved, err)
	}
	return comparison.Parse(resolved)
}

// Save writes the given AnalysisResult to the baseline path formatted with indentation.
func Save(path string, r *analysis.AnalysisResult) error {
	if r == nil {
		return fmt.Errorf("cannot save nil analysis result")
	}
	resolved := ResolvePath(path)
	dir := filepath.Dir(resolved)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %q: %w", dir, err)
		}
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal baseline JSON: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(resolved, data, 0644); err != nil {
		return fmt.Errorf("write baseline to %q: %w", resolved, err)
	}
	return nil
}
