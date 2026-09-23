// Package discovery automatically detects Angular esbuild build artifacts.
package discovery

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/snapshot"
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
		var exactMatches []Candidate
		for _, c := range candidates {
			if strings.EqualFold(c.Project, projectName) {
				exactMatches = append(exactMatches, c)
			}
		}
		if len(exactMatches) == 1 {
			return exactMatches[0].Stats, exactMatches[0].Dist, nil
		}
		if len(exactMatches) > 1 {
			candidates = exactMatches
		} else {
			var substringMatches []Candidate
			for _, c := range candidates {
				if strings.Contains(strings.ToLower(c.Project), strings.ToLower(projectName)) || strings.Contains(strings.ToLower(c.Stats), strings.ToLower(projectName)) {
					substringMatches = append(substringMatches, c)
				}
			}
			if len(substringMatches) == 1 {
				return substringMatches[0].Stats, substringMatches[0].Dist, nil
			}
			if len(substringMatches) == 0 {
				available := make([]string, 0, len(candidates))
				for _, c := range candidates {
					available = append(available, c.Project)
				}
				return "", "", fmt.Errorf("project %q not found among build artifacts; available projects: %s", projectName, strings.Join(available, ", "))
			}
			candidates = substringMatches
		}
	}

	if len(candidates) == 1 {
		return candidates[0].Stats, candidates[0].Dist, nil
	}

	// Multiple candidates found without unambiguous match
	var names []string
	for _, c := range candidates {
		names = append(names, fmt.Sprintf("%s (stats: %s, dist: %s)", c.Project, c.Stats, c.Dist))
	}
	firstProject := candidates[0].Project
	return "", "", fmt.Errorf("multiple Angular build outputs found:\n  - %s\nSpecify which project to analyze using 'bundleradar summary --project %s', or run 'bundleradar workspace summary'", strings.Join(names, "\n  - "), firstProject)
}

// Resolve determines the stats.json and browser dist paths based on provided inputs and workspace discovery.
// Precedence and resolution rules:
// 1. If stats points directly to an existing file, that file is authoritative and does not search unrelated workspace builds.
//    If dist is omitted, only the stats file's immediate sibling or child browser directory is inspected.
// 2. If both stats and dist are explicitly specified, they are validated and used.
// 3. If stats is an existing directory, discovery is scoped to that directory.
// 4. If stats is a non-existent path without a .json extension, it is treated as a project name filter.
// 5. If no artifacts can be resolved unambiguously, an error with available projects and the exact recovery command is returned.
func Resolve(rootDir, stats, dist, project string) (string, string, error) {
	if rootDir == "" {
		rootDir = "."
	}
	absPath := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(rootDir, p)
	}

	// 1. If stats points directly to an existing file, resolve dist deterministically
	if stats != "" {
		pStats := absPath(stats)
		if fi, err := os.Stat(pStats); err == nil && !fi.IsDir() {
			stats = pStats
			if dist != "" {
				return stats, absPath(dist), nil
			}
			dir := filepath.Dir(stats)
			browserSub := filepath.Join(dir, "browser")
			if bi, err := os.Stat(browserSub); err == nil && bi.IsDir() {
				return stats, browserSub, nil
			}
			return stats, dir, nil
		}
	}

	// 2. If both stats and dist are explicitly specified
	if stats != "" && dist != "" {
		pStats := absPath(stats)
		pDist := absPath(dist)
		if fi, err := os.Stat(pStats); err == nil && fi.IsDir() {
			if _, errDist := os.Stat(pDist); os.IsNotExist(errDist) && project == "" {
				return Locate(pStats, dist)
			}
		}
		return pStats, pDist, nil
	}

	// 3. Positional project or directory in stats argument
	searchDir := rootDir
	if stats != "" && dist == "" {
		pStats := absPath(stats)
		if fi, err := os.Stat(pStats); err == nil && fi.IsDir() {
			searchDir = pStats
			stats = ""
		} else if _, err := os.Stat(pStats); os.IsNotExist(err) && project == "" && !strings.HasSuffix(strings.ToLower(stats), ".json") {
			project = stats
			stats = ""
		}
	}

	// 4. Auto-discovery
	discoveredStats, discoveredDist, err := Locate(searchDir, project)
	if err != nil {
		if stats != "" && dist == "" {
			return "", "", fmt.Errorf("missing --dist directory path: %w", err)
		}
		if dist != "" && stats == "" {
			return "", "", fmt.Errorf("missing --stats file path: %w", err)
		}
		return "", "", err
	}
	if stats == "" {
		stats = discoveredStats
	}
	if dist == "" {
		dist = discoveredDist
	}
	return stats, dist, nil
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
			if name == "node_modules" || name == ".git" || name == ".bundleradar" {
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

	// Deduplicate candidates by project name: prefer final dist outputs over .nx/cache outputs
	candidateMap := make(map[string]Candidate)
	for _, c := range candidates {
		existing, ok := candidateMap[c.Project]
		if !ok {
			candidateMap[c.Project] = c
			continue
		}
		existingIsCache := strings.Contains(filepath.ToSlash(existing.Stats), "/.nx/cache/") || strings.HasPrefix(filepath.ToSlash(existing.Stats), ".nx/cache/")
		currentIsCache := strings.Contains(filepath.ToSlash(c.Stats), "/.nx/cache/") || strings.HasPrefix(filepath.ToSlash(c.Stats), ".nx/cache/")
		if existingIsCache && !currentIsCache {
			candidateMap[c.Project] = c
		}
	}

	deduped := make([]Candidate, 0, len(candidateMap))
	for _, c := range candidateMap {
		deduped = append(deduped, c)
	}

	slices.SortFunc(deduped, func(a, b Candidate) int {
		if cmp := strings.Compare(a.Project, b.Project); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Stats, b.Stats)
	})

	return deduped, nil
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
