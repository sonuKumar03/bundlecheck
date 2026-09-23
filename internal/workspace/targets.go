package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/discovery"
)

// AppTarget represents an explicitly declared or discovered application bundle target.
type AppTarget struct {
	Name  string `json:"name"`
	Stats string `json:"stats"`
	Dist  string `json:"dist"`
}

// ParseAppTargets parses and validates application targets from --projects and --app specs.
func ParseAppTargets(rootDir string, projectNames []string, appSpecs []string) ([]AppTarget, error) {
	if rootDir == "" {
		var err error
		rootDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve root dir: %w", err)
	}

	var targets []AppTarget
	seenNames := make(map[string]bool)
	seenStats := make(map[string]string)

	add := func(target AppTarget) error {
		canonicalName := strings.ToLower(strings.TrimSpace(target.Name))
		if canonicalName == "" {
			return fmt.Errorf("app target name cannot be empty")
		}
		if seenNames[canonicalName] {
			return fmt.Errorf("duplicate project name %q", target.Name)
		}

		cleanStats := strings.TrimSpace(target.Stats)
		if cleanStats == "" {
			return fmt.Errorf("stats file path for %q cannot be empty", target.Name)
		}
		if !filepath.IsAbs(cleanStats) {
			cleanStats = filepath.Join(absRoot, cleanStats)
		}
		cleanStats = filepath.Clean(cleanStats)

		fi, err := os.Stat(cleanStats)
		if err != nil {
			return fmt.Errorf("stats file for %q not found: %w", target.Name, err)
		}
		if fi.IsDir() {
			return fmt.Errorf("stats path for %q is a directory, expected file: %s", target.Name, cleanStats)
		}

		canonicalStats, err := filepath.EvalSymlinks(cleanStats)
		if err != nil {
			canonicalStats = cleanStats
		}
		if owner, exists := seenStats[canonicalStats]; exists {
			return fmt.Errorf("stats file %q is claimed by multiple projects (%q and %q)", cleanStats, owner, target.Name)
		}

		cleanDist := strings.TrimSpace(target.Dist)
		if cleanDist == "" {
			statsDir := filepath.Dir(cleanStats)
			browserDir := filepath.Join(statsDir, "browser")
			if bfi, err := os.Stat(browserDir); err == nil && bfi.IsDir() {
				cleanDist = browserDir
			} else {
				cleanDist = statsDir
			}
		} else if !filepath.IsAbs(cleanDist) {
			cleanDist = filepath.Join(absRoot, cleanDist)
		}
		cleanDist = filepath.Clean(cleanDist)

		target.Stats = cleanStats
		target.Dist = cleanDist
		targets = append(targets, target)
		seenNames[canonicalName] = true
		seenStats[canonicalStats] = target.Name
		return nil
	}

	// 1. Process explicit --app specs (name=stats[:dist])
	for _, spec := range appSpecs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		eqIdx := strings.Index(spec, "=")
		if eqIdx <= 0 {
			return nil, fmt.Errorf("invalid --app specification %q: expected format name=stats_path[:dist_path]", spec)
		}
		name := strings.TrimSpace(spec[:eqIdx])
		paths := strings.TrimSpace(spec[eqIdx+1:])
		if paths == "" {
			return nil, fmt.Errorf("invalid --app specification %q: missing stats path", spec)
		}
		var statsPath, distPath string
		colonIdx := strings.Index(paths, ":")
		if colonIdx >= 0 {
			statsPath = paths[:colonIdx]
			distPath = paths[colonIdx+1:]
		} else {
			statsPath = paths
		}

		if err := add(AppTarget{Name: name, Stats: statsPath, Dist: distPath}); err != nil {
			return nil, err
		}
	}

	// 2. Process convention --projects
	for _, proj := range projectNames {
		proj = strings.TrimSpace(proj)
		if proj == "" {
			continue
		}
		statsPath, distPath, err := discovery.Locate(absRoot, proj)
		if err != nil {
			return nil, fmt.Errorf("locate project %q: %w", proj, err)
		}
		if err := add(AppTarget{Name: proj, Stats: statsPath, Dist: distPath}); err != nil {
			return nil, err
		}
	}

	slices.SortFunc(targets, func(a, b AppTarget) int {
		return strings.Compare(a.Name, b.Name)
	})

	return targets, nil
}
