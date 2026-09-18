package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrStaticFallbackRequired is returned when static parsing cannot satisfy the workspace model.
var ErrStaticFallbackRequired = errors.New("static parsing cannot satisfy workspace model; Nx CLI fallback required")

type rawProjectFile struct {
	Name        string            `json:"name"`
	ProjectType string            `json:"projectType"`
	Targets     map[string]Target `json:"targets"`
}

// ReadMetadataStatic scans the workspace root for modern project.json manifests without running Node.js.
func ReadMetadataStatic(root string) (Metadata, error) {
	var m Metadata
	m.Graph.Nodes = make(map[string]Project)

	fi, err := os.Stat(filepath.Join(root, "nx.json"))
	if err != nil {
		return m, fmt.Errorf("nx.json not found in %s: %w", root, err)
	}
	if fi.IsDir() {
		return m, fmt.Errorf("nx.json is a directory in %s", root)
	}

	cleanRoot := filepath.Clean(root)
	maxDepth := 4

	err = filepath.WalkDir(cleanRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if path != cleanRoot && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "dist" || name == "coverage" || name == "tmp") {
				return filepath.SkipDir
			}

			rel, relErr := filepath.Rel(cleanRoot, path)
			if relErr == nil && rel != "." {
				depth := len(strings.Split(rel, string(filepath.Separator)))
				if depth > maxDepth {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if d.Name() == "project.json" {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			var pf rawProjectFile
			if err := json.Unmarshal(data, &pf); err != nil {
				return nil
			}

			dir := filepath.Dir(path)
			relRoot, err := filepath.Rel(cleanRoot, dir)
			if err != nil {
				return nil
			}

			projName := pf.Name
			if projName == "" {
				projName = filepath.Base(dir)
			}

			p := Project{
				Type: "lib",
			}
			pType := strings.ToLower(pf.ProjectType)
			if pType == "application" || pType == "app" {
				p.Type = "app"
			}
			p.Data.Root = filepath.ToSlash(relRoot)
			p.Data.ProjectType = pf.ProjectType
			if p.Data.ProjectType == "" {
				if p.Type == "app" {
					p.Data.ProjectType = "application"
				} else {
					p.Data.ProjectType = "library"
				}
			}
			if pf.Targets != nil {
				p.Data.Targets = pf.Targets
			} else {
				p.Data.Targets = make(map[string]Target)
			}

			m.Graph.Nodes[projName] = p
		}
		return nil
	})

	if err != nil {
		return m, err
	}

	if len(m.Graph.Nodes) == 0 {
		return m, ErrStaticFallbackRequired
	}

	hasSupportedApp := false
	for _, p := range m.Graph.Nodes {
		if IsApplication(p) {
			for targetName := range p.Data.Targets {
				if Supported(p, targetName) {
					hasSupportedApp = true
					break
				}
			}
			if hasSupportedApp {
				break
			}
		}
	}

	if !hasSupportedApp {
		return m, ErrStaticFallbackRequired
	}

	return m, nil
}
