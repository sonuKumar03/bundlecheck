package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ResolveConfiguredEntry finds the Angular build browser/main source entry
// associated with a stats file without executing workspace code.
func ResolveConfiguredEntry(statsFile, project string) (string, bool) {
	for dir := filepath.Dir(statsFile); ; dir = filepath.Dir(dir) {
		if entry, ok := entryFromAngularJSON(filepath.Join(dir, "angular.json"), project); ok {
			return entry, true
		}
		if _, err := os.Stat(filepath.Join(dir, "nx.json")); err == nil {
			if metadata, err := ReadMetadataStatic(dir); err == nil {
				if p, ok := selectProject(metadata.Graph.Nodes, project); ok {
					return entryFromTargets(p.Data.Targets)
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
	}
}

func entryFromAngularJSON(path, projectName string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var config struct {
		Projects map[string]struct {
			Targets   map[string]Target `json:"targets"`
			Architect map[string]Target `json:"architect"`
		} `json:"projects"`
	}
	if json.Unmarshal(data, &config) != nil {
		return "", false
	}
	if projectName == "" && len(config.Projects) == 1 {
		for name := range config.Projects {
			projectName = name
		}
	}
	p, ok := config.Projects[projectName]
	if !ok {
		return "", false
	}
	if p.Targets == nil {
		p.Targets = p.Architect
	}
	return entryFromTargets(p.Targets)
}

func entryFromTargets(targets map[string]Target) (string, bool) {
	target, ok := targets["build"]
	if !ok {
		return "", false
	}
	entries := make(map[string]bool)
	addEntry := func(options map[string]json.RawMessage) {
		for _, key := range []string{"browser", "main"} {
			var entry string
			if json.Unmarshal(options[key], &entry) == nil && entry != "" {
				entries[filepath.ToSlash(entry)] = true
				return
			}
		}
	}
	addEntry(target.Options)
	for _, overrides := range target.Configurations {
		options := make(map[string]json.RawMessage, len(target.Options)+len(overrides))
		for key, value := range target.Options {
			options[key] = value
		}
		for key, value := range overrides {
			options[key] = value
		}
		addEntry(options)
	}
	if len(entries) != 1 {
		return "", false
	}
	for entry := range entries {
		return entry, true
	}
	return "", false
}

func selectProject(projects map[string]Project, name string) (Project, bool) {
	if name != "" {
		p, ok := projects[name]
		return p, ok
	}
	if len(projects) != 1 {
		return Project{}, false
	}
	for _, p := range projects {
		return p, true
	}
	return Project{}, false
}
