// Package workspace reads Nx metadata and aggregates existing browser builds.
package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type Target struct {
	Executor       string                                `json:"executor"`
	Options        map[string]json.RawMessage            `json:"options"`
	Configurations map[string]map[string]json.RawMessage `json:"configurations"`
}
type Project struct {
	Type string `json:"type"`
	Data struct {
		Root        string            `json:"root"`
		ProjectType string            `json:"projectType"`
		Targets     map[string]Target `json:"targets"`
	} `json:"data"`
}
type Metadata struct {
	Graph struct {
		Nodes map[string]Project `json:"nodes"`
	} `json:"graph"`
}

func Root(start string, explicit bool) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if fi, err := os.Stat(filepath.Join(dir, "nx.json")); err == nil && !fi.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if explicit || parent == dir {
			return "", fmt.Errorf("no nx.json found from %q; use --root to select an Nx workspace", start)
		}
		dir = parent
	}
}

func ReadMetadata(ctx context.Context, root string) (Metadata, error) {
	var m Metadata
	nxDir := filepath.Join(root, "node_modules", "nx")
	manifest, err := os.ReadFile(filepath.Join(nxDir, "package.json"))
	if err != nil {
		return m, fmt.Errorf("workspace-installed Nx not found; install the workspace dependencies first: %w", err)
	}
	var pkg struct {
		Bin map[string]string `json:"bin"`
	}
	if err := json.Unmarshal(manifest, &pkg); err != nil {
		return m, fmt.Errorf("invalid installed Nx package manifest: %w", err)
	}
	bin := filepath.FromSlash(pkg.Bin["nx"])
	if bin == "" || filepath.IsAbs(bin) || bin == ".." || strings.HasPrefix(filepath.Clean(bin), ".."+string(filepath.Separator)) {
		return m, fmt.Errorf("installed Nx package has no usable nx CLI entry")
	}
	cli := filepath.Join(nxDir, bin)
	if _, err := os.Stat(cli); err != nil {
		return m, fmt.Errorf("installed Nx CLI missing: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", cli, "graph", "--print")
	command.Dir = root
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "NX_DAEMON=") && !strings.HasPrefix(entry, "NX_INTERACTIVE=") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "NX_DAEMON=false", "NX_INTERACTIVE=false")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return m, fmt.Errorf("read Nx project graph (requires local Node and Nx): %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if err = json.Unmarshal(output, &m); err != nil {
		return m, fmt.Errorf("invalid Nx graph JSON: %w", err)
	}
	if len(m.Graph.Nodes) == 0 {
		return m, fmt.Errorf("nx graph has no project nodes; require Nx supporting 'graph --print'")
	}
	return m, nil
}

func IsApplication(p Project) bool { return p.Type == "app" || p.Data.ProjectType == "application" }
func Supported(p Project, target string) bool {
	switch p.Data.Targets[target].Executor {
	case "@nx/angular:application", "@nx/angular:browser-esbuild", "@angular-devkit/build-angular:application", "@angular-devkit/build-angular:browser-esbuild", "@angular/build:application":
		return true
	}
	return false
}

func OutputDirectories(root string, p Project, target, configuration string) (string, string, error) {
	t, ok := p.Data.Targets[target]
	if !ok {
		return "", "", fmt.Errorf("target %q not found", target)
	}
	overrides, ok := t.Configurations[configuration]
	if !ok {
		return "", "", fmt.Errorf("configuration %q not found on target %q", configuration, target)
	}
	raw := t.Options["outputPath"]
	if value, exists := overrides["outputPath"]; exists {
		raw = value
	}
	var base string
	browser := ""
	if strings.HasSuffix(t.Executor, ":application") {
		browser = "browser"
	}
	if err := json.Unmarshal(raw, &base); err != nil {
		var object struct {
			Base    string  `json:"base"`
			Browser *string `json:"browser"`
		}
		if err := json.Unmarshal(raw, &object); err != nil {
			return "", "", fmt.Errorf("outputPath must be a string or an object with base and browser")
		}
		base = object.Base
		if object.Browser != nil {
			browser = *object.Browser
		}
	}
	if base == "" {
		return "", "", fmt.Errorf("target has no usable outputPath")
	}
	if browser != "" && (browser == "." || browser == ".." || strings.ContainsAny(browser, `/\`)) {
		return "", "", fmt.Errorf("invalid browser output directory %q", browser)
	}
	base = strings.ReplaceAll(base, `\`, "/")
	if !filepath.IsAbs(base) {
		base = filepath.Join(root, filepath.FromSlash(base))
	}
	return filepath.Clean(base), filepath.Join(base, browser), nil
}

// Artifacts examines only the configured browser directory and its output base.
// Server and sibling app directories cannot supply a matching artifact.
func Artifacts(base, browser string) (string, string, error) {
	if fi, err := os.Stat(filepath.Join(browser, "index.html")); err != nil || fi.IsDir() {
		return "", "", fmt.Errorf("missing browser index.html in %s; build with --stats-json first", browser)
	}
	var stats []string
	dirs := []string{base}
	if browser != base {
		dirs = append(dirs, browser)
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return "", "", err
		}
		for _, entry := range entries {
			if !entry.IsDir() && (entry.Name() == "stats.json" || strings.HasSuffix(entry.Name(), ".stats.json")) {
				stats = append(stats, filepath.Join(dir, entry.Name()))
			}
		}
	}
	slices.Sort(stats)
	if len(stats) != 1 {
		return "", "", fmt.Errorf("expected one stats file in %s or %s, found %d; build with --stats-json and remove stale stats files", base, browser, len(stats))
	}
	return stats[0], browser, nil
}
