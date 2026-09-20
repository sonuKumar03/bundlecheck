package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

func CheckFreshness(root, projectRoot, stats string, s *snapshot.BundleSnapshot) *Freshness {
	result := &Freshness{Status: "unknown"}
	artifact, err := os.Stat(stats)
	if err != nil {
		return result
	}
	artifactTime := artifact.ModTime()
	result.ArtifactModifiedAt = artifactTime.UTC().Format(time.RFC3339)

	candidates := []string{
		filepath.Join(projectRoot, "project.json"),
		"angular.json", "nx.json", "package.json", "package-lock.json",
		"pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb", "tsconfig.base.json",
	}
	if s != nil {
		for _, input := range s.Inputs {
			path := snapshot.CleanPath(input.Path)
			if path != "" && !strings.HasPrefix(path, "node_modules/") && !strings.HasPrefix(path, "<") {
				candidates = append(candidates, path)
			}
		}
	}

	var newest time.Time
	seen := map[string]bool{}
	for _, candidate := range candidates {
		path, name, ok := freshnessPath(root, candidate)
		if !ok || seen[path] {
			continue
		}
		seen[path] = true
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || !info.ModTime().After(artifactTime) || !info.ModTime().After(newest) {
			continue
		}
		newest = info.ModTime()
		result.Status = "stale-suspected"
		result.NewestInput = name
		result.NewestInputModifiedAt = newest.UTC().Format(time.RFC3339)
	}
	return result
}

func freshnessPath(root, candidate string) (string, string, bool) {
	path := filepath.FromSlash(candidate)
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", false
	}
	return absPath, filepath.ToSlash(rel), true
}
