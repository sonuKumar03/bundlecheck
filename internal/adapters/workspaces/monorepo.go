package workspaces

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
	"gopkg.in/yaml.v3"
)

// MonorepoResolver discovers targets across standard npm/pnpm/yarn workspaces.
type MonorepoResolver struct{}

func (r *MonorepoResolver) Name() string {
	return "monorepo"
}

func (r *MonorepoResolver) Detect(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "pnpm-workspace.yaml")); err == nil {
		return true
	}
	pkgPath := filepath.Join(root, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var p struct {
			Workspaces any `json:"workspaces"`
		}
		if err := json.Unmarshal(data, &p); err == nil && p.Workspaces != nil {
			return true
		}
	}
	return false
}

func (r *MonorepoResolver) Resolve(ctx context.Context, root string) ([]core.Target, error) {
	globs := []string{"apps/*", "packages/*"}

	pnpmPath := filepath.Join(root, "pnpm-workspace.yaml")
	if data, err := os.ReadFile(pnpmPath); err == nil {
		var p struct {
			Packages []string `yaml:"packages"`
		}
		if err := yaml.Unmarshal(data, &p); err == nil && len(p.Packages) > 0 {
			globs = p.Packages
		}
	}

	var targets []core.Target
	seen := make(map[string]bool)

	for _, g := range globs {
		matches, err := filepath.Glob(filepath.Join(root, g))
		if err != nil {
			continue
		}
		for _, dir := range matches {
			fi, err := os.Stat(dir)
			if err != nil || !fi.IsDir() {
				continue
			}

			// Read project name from package.json or folder name
			name := filepath.Base(dir)
			pkgJSONPath := filepath.Join(dir, "package.json")
			if pdata, err := os.ReadFile(pkgJSONPath); err == nil {
				var p struct {
					Name string `json:"name"`
				}
				if err := json.Unmarshal(pdata, &p); err == nil && p.Name != "" {
					// Use base package name or clean name
					name = p.Name
					if idx := strings.Index(name, "/"); idx != -1 {
						name = name[idx+1:]
					}
				}
			}

			if seen[name] {
				continue
			}

			// Locate stats.json or build output
			statsCandidates := []string{
				filepath.Join(dir, "dist", "stats.json"),
				filepath.Join(dir, "stats.json"),
				filepath.Join(dir, "dist", "metafile.json"),
				filepath.Join(dir, "dist", "manifest.json"),
			}

			for _, sc := range statsCandidates {
				if sfi, err := os.Stat(sc); err == nil && !sfi.IsDir() {
					distDir := filepath.Dir(sc)
					if bfi, err := os.Stat(filepath.Join(distDir, "browser")); err == nil && bfi.IsDir() {
						distDir = filepath.Join(distDir, "browser")
					}
					targets = append(targets, core.Target{
						Name:      name,
						StatsPath: sc,
						DistPath:  distDir,
					})
					seen[name] = true
					break
				}
			}
		}
	}

	return targets, nil
}
