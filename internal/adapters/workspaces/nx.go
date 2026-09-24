package workspaces

import (
	"context"
	"os"
	"path/filepath"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

// NxResolver discovers projects in Nx workspaces without calling the Nx CLI.
type NxResolver struct{}

func (r *NxResolver) Name() string {
	return "nx"
}

func (r *NxResolver) Detect(root string) bool {
	_, err := os.Stat(filepath.Join(root, "nx.json"))
	return err == nil
}

func (r *NxResolver) Resolve(ctx context.Context, root string) ([]core.Target, error) {
	// Look inside apps/
	appsDir := filepath.Join(root, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, nil
	}

	var targets []core.Target
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		appPath := filepath.Join(appsDir, name)

		// Look for standard dist or mock stats
		statsCandidates := []string{
			filepath.Join(root, "dist", "apps", name, "stats.json"),
			filepath.Join(root, "dist", "apps", name, "browser", "stats.json"),
			filepath.Join(root, "dist", "apps", name, "metafile.json"),
			filepath.Join(root, "dist", "apps", name, "manifest.json"),
			filepath.Join(root, "dist", "apps", name, ".vite", "manifest.json"),
			filepath.Join(appPath, "dist", "stats.json"),
			filepath.Join(appPath, "stats.json"),
			filepath.Join(appPath, "dist", "metafile.json"),
			filepath.Join(appPath, "dist", "manifest.json"),
			filepath.Join(appPath, "dist", ".vite", "manifest.json"),
		}

		foundStats := ""
		foundDist := ""
		for _, sc := range statsCandidates {
			if _, err := os.Stat(sc); err == nil {
				foundStats = sc
				foundDist = filepath.Dir(sc)
				browserSub := filepath.Join(foundDist, "browser")
				if fi, err := os.Stat(browserSub); err == nil && fi.IsDir() {
					foundDist = browserSub
				}
				break
			}
		}

		if foundStats == "" {
			// Even if unbuilt, record project target
			foundStats = filepath.Join(root, "dist", "apps", name, "stats.json")
			foundDist = filepath.Join(root, "dist", "apps", name, "browser")
		}

		targets = append(targets, core.Target{
			Name:      name,
			StatsPath: foundStats,
			DistPath:  foundDist,
		})
	}

	return targets, nil
}
