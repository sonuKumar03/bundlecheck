// Package discovery automatically detects Angular esbuild build artifacts.
package discovery

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

type Candidate struct {
	Project string
	Stats   string
	Dist    string
}

// Locate finds Angular esbuild build artifacts (stats.json and browser dist)
// in the given root directory (usually working directory).
// If projectName is non-empty, it filters candidates matching the project name.
func Locate(rootDir string, projectName string) (string, string, error) {
	candidates, err := FindCandidates(rootDir)
	if err != nil {
		return "", "", err
	}

	if len(candidates) == 0 {
		return "", "", fmt.Errorf("no Angular build artifacts found in %q\nHint: run 'ng build --configuration production --stats-json' first, or supply --stats and --dist explicitly", rootDir)
	}

	if projectName != "" {
		filtered := []Candidate{}
		for _, c := range candidates {
			if strings.EqualFold(c.Project, projectName) || strings.Contains(strings.ToLower(c.Stats), strings.ToLower(projectName)) {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) == 1 {
			return filtered[0].Stats, filtered[0].Dist, nil
		}
		if len(filtered) == 0 {
			available := make([]string, 0, len(candidates))
			for _, c := range candidates {
				available = append(available, c.Project)
			}
			return "", "", fmt.Errorf("project %q not found among build artifacts; available projects: %s", projectName, strings.Join(available, ", "))
		}
		candidates = filtered
	}

	if len(candidates) == 1 {
		return candidates[0].Stats, candidates[0].Dist, nil
	}

	// Multiple candidates found without unambiguous match
	var names []string
	for _, c := range candidates {
		names = append(names, fmt.Sprintf("%s (stats: %s, dist: %s)", c.Project, c.Stats, c.Dist))
	}
	return "", "", fmt.Errorf("multiple Angular build outputs found:\n  - %s\nSpecify which project to analyze using 'bundlecheck summary <project>' (or --project <name>), or run 'bundlecheck workspace summary'", strings.Join(names, "\n  - "))
}

// FindCandidates searches for matching stats.json and browser dist directories.
func FindCandidates(rootDir string) ([]Candidate, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve root dir: %w", err)
	}

	distDir := filepath.Join(absRoot, "dist")
	searchRoot := distDir
	if fi, err := os.Stat(distDir); err != nil || !fi.IsDir() {
		searchRoot = absRoot
	}

	var statsFiles []string
	var indexDirs []string

	_ = filepath.WalkDir(searchRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Avoid walking node_modules or .git
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == ".git" || name == ".bundlecheck" {
				return filepath.SkipDir
			}
			return nil
		}

		name := d.Name()
		if name == "stats.json" || strings.HasSuffix(name, ".stats.json") {
			statsFiles = append(statsFiles, p)
		} else if (name == "index.html" || name == "index.csr.html") && filepath.Base(filepath.Dir(p)) != "server" {
			dir := filepath.Dir(p)
			if hasJavaScriptFiles(dir) {
				indexDirs = append(indexDirs, dir)
			}
		}
		return nil
	})

	var candidates []Candidate
	seen := make(map[string]bool)

	// Index candidate browser directories by exact directory and parent directory for O(1) lookups
	indexDirsByDir := make(map[string][]string, len(indexDirs))
	indexDirsByParent := make(map[string][]string, len(indexDirs))

	for _, dist := range indexDirs {
		indexDirsByDir[dist] = append(indexDirsByDir[dist], dist)
		parent := filepath.Dir(dist)
		indexDirsByParent[parent] = append(indexDirsByParent[parent], dist)
	}

	for _, stats := range statsFiles {
		statsDir := filepath.Dir(stats)
		statsParent := filepath.Dir(statsDir)

		// Collect related candidate directories from pre-indexed maps
		var relatedDists []string
		relatedDists = append(relatedDists, indexDirsByDir[statsDir]...)
		relatedDists = append(relatedDists, indexDirsByParent[statsDir]...)
		if statsParent != statsDir {
			relatedDists = append(relatedDists, indexDirsByDir[statsParent]...)
			relatedDists = append(relatedDists, indexDirsByParent[statsParent]...)
		}

		for _, dist := range relatedDists {
			if isRelated(statsDir, dist) {
				key := stats + "::" + dist
				if !seen[key] {
					seen[key] = true
					project := deriveProjectName(absRoot, stats, dist)
					candidates = append(candidates, Candidate{
						Project: project,
						Stats:   stats,
						Dist:    dist,
					})
				}
			}
		}
	}

	slices.SortFunc(candidates, func(a, b Candidate) int {
		return strings.Compare(a.Project, b.Project)
	})

	return candidates, nil
}

func hasJavaScriptFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && snapshot.IsJavaScript(e.Name()) {
			return true
		}
	}
	return false
}

func isRelated(statsDir, distDir string) bool {
	if statsDir == distDir {
		return true
	}
	if filepath.Dir(distDir) == statsDir {
		return true
	}
	if filepath.Dir(statsDir) == distDir {
		return true
	}
	if filepath.Dir(statsDir) == filepath.Dir(distDir) {
		return true
	}
	return false
}

func deriveProjectName(rootDir, statsPath, distPath string) string {
	rel, err := filepath.Rel(rootDir, statsPath)
	if err != nil {
		rel = statsPath
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, p := range parts {
		if p != "dist" && p != "apps" && p != "projects" && p != "browser" && p != "stats.json" && p != "." && p != "" && !strings.HasSuffix(p, ".json") {
			return p
		}
	}
	return filepath.Base(filepath.Dir(distPath))
}
