// Package baseline manages saving, loading, and managing baseline summary metrics and git snapshots.
package baseline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/comparison"
	"bundlecheck/internal/snapshot"
)

const (
	DefaultDir              = ".bundlecheck"
	DefaultBaselinesSubdir  = "baselines"
	DefaultActiveFilename   = "active"
	DefaultBaselineFilename = ".bundlecheck/baseline.json"
)

// Metadata captures provenance for a baseline (git ref, commit, build command, timestamp).
type Metadata struct {
	Name      string    `json:"name,omitempty"`
	GitRef    string    `json:"gitRef,omitempty"`
	CommitSHA string    `json:"commitSha,omitempty"`
	BuildCmd  string    `json:"buildCmd,omitempty"`
	Project   string    `json:"project,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

// BaselineFile is the wire representation of a baseline snapshot.
type BaselineFile struct {
	SchemaVersion string             `json:"schemaVersion"`
	ToolVersion   string             `json:"toolVersion"`
	Command       string             `json:"command"`
	Summary       snapshot.Totals    `json:"summary"`
	Packages      []snapshot.Package `json:"packages"`
	Metadata      *Metadata          `json:"metadata,omitempty"`
}

// BaselineInfo provides a high-level summary of a saved baseline for listing.
type BaselineInfo struct {
	Name      string    `json:"name"`
	GitRef    string    `json:"gitRef,omitempty"`
	CommitSHA string    `json:"commitSha,omitempty"`
	InitialJS int64     `json:"initialJs"`
	LazyJS    int64     `json:"lazyJs"`
	TotalJS   int64     `json:"totalJs"`
	CreatedAt time.Time `json:"createdAt"`
	Path      string    `json:"path"`
	IsActive  bool      `json:"isActive"`
}

// ResolvePath returns the provided path, or resolves a baseline name / active default.
func ResolvePath(customPath string) string {
	if customPath != "" {
		// If customPath is a file or contains path separators, use it directly
		if strings.Contains(customPath, "/") || strings.Contains(customPath, "\\") || strings.HasSuffix(customPath, ".json") {
			return customPath
		}
		// Otherwise resolve named baseline
		namedPath := filepath.Join(DefaultDir, DefaultBaselinesSubdir, customPath+".json")
		if _, err := os.Stat(namedPath); err == nil {
			return namedPath
		}
		return customPath
	}

	// If active pointer exists, resolve active baseline
	activeName, err := GetActive(DefaultDir)
	if err == nil && activeName != "" {
		namedPath := filepath.Join(DefaultDir, DefaultBaselinesSubdir, activeName+".json")
		if _, err := os.Stat(namedPath); err == nil {
			return namedPath
		}
	}

	return DefaultBaselineFilename
}

// Load reads and parses a saved baseline summary JSON document.
func Load(pathOrName string) (*analysis.AnalysisResult, error) {
	resolved := ResolvePath(pathOrName)
	if _, err := os.Stat(resolved); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("baseline %q not found\nHint: run 'bundlecheck baseline' or 'bundlecheck baseline save' first", resolved)
		}
		return nil, fmt.Errorf("read baseline %q: %w", resolved, err)
	}
	return comparison.Parse(resolved)
}

// LoadSnapshot reads both the analysis result and metadata from a baseline file.
func LoadSnapshot(pathOrName string) (*BaselineFile, error) {
	resolved := ResolvePath(pathOrName)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("read baseline %q: %w", resolved, err)
	}

	var bf BaselineFile
	if err := json.Unmarshal(data, &bf); err != nil {
		return nil, fmt.Errorf("invalid baseline JSON in %q: %w", resolved, err)
	}

	res, err := comparison.Parse(resolved)
	if err != nil {
		return nil, err
	}

	bf.Summary = res.Summary
	bf.Packages = res.Packages
	if bf.Metadata == nil {
		bf.Metadata = &Metadata{
			Name: strings.TrimSuffix(filepath.Base(resolved), ".json"),
		}
	}
	return &bf, nil
}

// Save writes the given AnalysisResult to the baseline path formatted with indentation.
func Save(path string, r *analysis.AnalysisResult) error {
	return SaveWithMetadata(path, r, nil)
}

// SaveWithMetadata writes the given AnalysisResult along with metadata.
func SaveWithMetadata(path string, r *analysis.AnalysisResult, meta *Metadata) error {
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

	bf := BaselineFile{
		SchemaVersion: r.SchemaVersion,
		ToolVersion:   r.ToolVersion,
		Command:       r.Command,
		Summary:       r.Summary,
		Packages:      r.Packages,
		Metadata:      meta,
	}

	data, err := json.MarshalIndent(bf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal baseline JSON: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(resolved, data, 0644); err != nil {
		return fmt.Errorf("write baseline to %q: %w", resolved, err)
	}
	return nil
}

// SaveNamed saves a baseline into the named baselines directory and optionally marks it as active.
func SaveNamed(baseDir string, name string, r *analysis.AnalysisResult, meta *Metadata) (string, error) {
	if strings.TrimSpace(name) == "" {
		name = "default"
	}
	if baseDir == "" {
		baseDir = DefaultDir
	}
	if meta == nil {
		meta = &Metadata{}
	}
	meta.Name = name
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now().UTC()
	}

	baselinesDir := filepath.Join(baseDir, DefaultBaselinesSubdir)
	if err := os.MkdirAll(baselinesDir, 0755); err != nil {
		return "", fmt.Errorf("create baselines directory %q: %w", baselinesDir, err)
	}

	targetPath := filepath.Join(baselinesDir, name+".json")
	if err := SaveWithMetadata(targetPath, r, meta); err != nil {
		return "", err
	}

	// Always set active when saving or updating default
	_ = SetActive(baseDir, name)

	return targetPath, nil
}

// SetActive sets the active baseline name and syncs DefaultBaselineFilename.
func SetActive(baseDir string, name string) error {
	if baseDir == "" {
		baseDir = DefaultDir
	}
	name = strings.TrimSpace(name)
	namedPath := filepath.Join(baseDir, DefaultBaselinesSubdir, name+".json")
	data, err := os.ReadFile(namedPath)
	if err != nil {
		return fmt.Errorf("baseline %q does not exist: %w", name, err)
	}

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return err
	}

	// Write active pointer
	activeFile := filepath.Join(baseDir, DefaultActiveFilename)
	if err := os.WriteFile(activeFile, []byte(name+"\n"), 0644); err != nil {
		return fmt.Errorf("write active baseline: %w", err)
	}

	// Sync default baseline.json for backward compatibility
	defaultBaseline := filepath.Join(baseDir, "baseline.json")
	_ = os.WriteFile(defaultBaseline, data, 0644)

	return nil
}

// GetActive returns the name of the currently active baseline.
func GetActive(baseDir string) (string, error) {
	if baseDir == "" {
		baseDir = DefaultDir
	}
	activeFile := filepath.Join(baseDir, DefaultActiveFilename)
	data, err := os.ReadFile(activeFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "default", nil
		}
		return "", err
	}
	name := strings.TrimSpace(string(data))
	if name == "" {
		return "default", nil
	}
	return name, nil
}

// List returns a list of all saved baselines and the active baseline name.
func List(baseDir string) ([]BaselineInfo, string, error) {
	if baseDir == "" {
		baseDir = DefaultDir
	}
	active, _ := GetActive(baseDir)

	baselinesDir := filepath.Join(baseDir, DefaultBaselinesSubdir)
	entries, err := os.ReadDir(baselinesDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Check if single legacy baseline exists
			legacyPath := filepath.Join(baseDir, "baseline.json")
			if fi, err := os.Stat(legacyPath); err == nil && !fi.IsDir() {
				res, err := comparison.Parse(legacyPath)
				if err == nil {
					return []BaselineInfo{
						{
							Name:      "default",
							InitialJS: res.Summary.InitialJS,
							LazyJS:    res.Summary.LazyJS,
							TotalJS:   res.Summary.TotalJS,
							CreatedAt: fi.ModTime().UTC(),
							Path:      legacyPath,
							IsActive:  true,
						},
					}, "default", nil
				}
			}
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("read baselines directory: %w", err)
	}

	var results []BaselineInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		filePath := filepath.Join(baselinesDir, entry.Name())

		bf, err := LoadSnapshot(filePath)
		if err != nil {
			continue
		}

		info := BaselineInfo{
			Name:      name,
			InitialJS: bf.Summary.InitialJS,
			LazyJS:    bf.Summary.LazyJS,
			TotalJS:   bf.Summary.TotalJS,
			Path:      filePath,
			IsActive:  name == active,
		}

		if bf.Metadata != nil {
			info.GitRef = bf.Metadata.GitRef
			info.CommitSHA = bf.Metadata.CommitSHA
			info.CreatedAt = bf.Metadata.CreatedAt
		}
		if info.CreatedAt.IsZero() {
			if fi, err := entry.Info(); err == nil {
				info.CreatedAt = fi.ModTime().UTC()
			}
		}

		results = append(results, info)
	}

	slices.SortFunc(results, func(a, b BaselineInfo) int {
		if a.IsActive != b.IsActive {
			if a.IsActive {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})

	return results, active, nil
}

// Delete removes a saved named baseline.
func Delete(baseDir string, name string) error {
	if baseDir == "" {
		baseDir = DefaultDir
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("baseline name is required")
	}

	targetPath := filepath.Join(baseDir, DefaultBaselinesSubdir, name+".json")
	if err := os.Remove(targetPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("baseline %q does not exist", name)
		}
		return fmt.Errorf("delete baseline %q: %w", name, err)
	}

	active, _ := GetActive(baseDir)
	if active == name {
		_ = os.Remove(filepath.Join(baseDir, DefaultActiveFilename))
	}

	return nil
}
