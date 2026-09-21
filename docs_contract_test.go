package main_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/sonuKumar03/bundlecheck/cmd"
	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/config"
)

// TestDocumentationContract_ActionInputs parses action.yml and verifies that all
// inputs referenced in README.md, npm/bundlecheck/README.md, and docs/index.html
// correspond to real, declared inputs in action.yml.
func TestDocumentationContract_ActionInputs(t *testing.T) {
	actionData, err := os.ReadFile("action.yml")
	if err != nil {
		t.Fatalf("failed to read action.yml: %v", err)
	}

	var actionDef struct {
		Inputs map[string]any `yaml:"inputs"`
	}
	if err := yaml.Unmarshal(actionData, &actionDef); err != nil {
		t.Fatalf("failed to parse action.yml: %v", err)
	}

	if len(actionDef.Inputs) == 0 {
		t.Fatal("action.yml has no inputs defined")
	}

	docFiles := []string{
		"README.md",
		filepath.Join("npm", "bundlecheck", "README.md"),
		filepath.Join("docs", "index.html"),
	}

	// Regex to extract with: blocks inside bundlecheck GitHub Action YAML examples
	withBlockRegex := regexp.MustCompile(`(?s)uses:\s*[^'\n]*bundlecheck[^\n]*\n\s*with:\s*\n((?:\s{8,14}[a-zA-Z0-9_-]+:\s*[^\n]*\n)+)`)
	keyRegex := regexp.MustCompile(`^\s*([a-zA-Z0-9_-]+):`)

	for _, docFile := range docFiles {
		content, err := os.ReadFile(docFile)
		if err != nil {
			t.Fatalf("failed to read %s: %v", docFile, err)
		}

		matches := withBlockRegex.FindAllStringSubmatch(string(content), -1)
		if len(matches) == 0 {
			continue
		}

		for _, match := range matches {
			lines := strings.Split(match[1], "\n")
			for _, line := range lines {
				line = strings.TrimRight(line, " \r\t")
				if line == "" {
					continue
				}
				keyMatch := keyRegex.FindStringSubmatch(line)
				if len(keyMatch) < 2 {
					continue
				}
				key := keyMatch[1]
				if _, ok := actionDef.Inputs[key]; !ok {
					t.Errorf("%s documents non-existent action input %q under with:", docFile, key)
				}
			}
		}
	}
}

// TestDocumentationContract_ConfigurationExamples ensures that every .bundlecheck.yml
// example in public documentation parses cleanly using the strict configuration loader.
func TestDocumentationContract_ConfigurationExamples(t *testing.T) {
	docFiles := []string{
		"README.md",
		filepath.Join("npm", "bundlecheck", "README.md"),
		filepath.Join("docs", "index.html"),
	}

	mdYamlRegex := regexp.MustCompile("(?s)```ya?ml\\s*\n(# \\.bundlecheck\\.yml.*?)```")
	htmlYamlRegex := regexp.MustCompile(`(?s)<pre><code>(# \.bundlecheck\.yml.*?)</code></pre>`)

	for _, docFile := range docFiles {
		content, err := os.ReadFile(docFile)
		if err != nil {
			t.Fatalf("failed to read %s: %v", docFile, err)
		}

		var snippets []string
		for _, match := range mdYamlRegex.FindAllStringSubmatch(string(content), -1) {
			snippets = append(snippets, match[1])
		}
		for _, match := range htmlYamlRegex.FindAllStringSubmatch(string(content), -1) {
			snippets = append(snippets, match[1])
		}

		for _, snippet := range snippets {
			var cfg config.Config
			decoder := yaml.NewDecoder(bytes.NewReader([]byte(snippet)))
			decoder.KnownFields(true)
			if err := decoder.Decode(&cfg); err != nil {
				t.Errorf("%s contains invalid or unsupported .bundlecheck.yml snippet:\n%s\nError: %v", docFile, snippet, err)
			}
		}
	}
}

// TestDocumentationContract_VersionSync validates version alignment across repo artifacts.
func TestDocumentationContract_VersionSync(t *testing.T) {
	rawVersion, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatalf("failed to read VERSION: %v", err)
	}
	version := strings.TrimSpace(string(rawVersion))
	if version == "" {
		t.Fatal("VERSION file is empty")
	}

	if analysis.ToolVersion != version {
		t.Errorf("analysis.ToolVersion (%s) != VERSION (%s)", analysis.ToolVersion, version)
	}

	// npm package version
	pkgData, err := os.ReadFile(filepath.Join("npm", "bundlecheck", "package.json"))
	if err != nil {
		t.Fatalf("failed to read package.json: %v", err)
	}
	var pkg struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(pkgData, &pkg); err != nil {
		t.Fatalf("failed to parse package.json: %v", err)
	}
	if pkg.Version != version {
		t.Errorf("npm package version (%s) != VERSION (%s)", pkg.Version, version)
	}
	if pkg.Name != "@sonukumar03/bundlecheck" {
		t.Errorf("expected scoped package name @sonukumar03/bundlecheck, got %s", pkg.Name)
	}

	// go.mod module path
	modData, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}
	if !strings.Contains(string(modData), "module github.com/sonuKumar03/bundlecheck") {
		t.Errorf("go.mod does not declare module github.com/sonuKumar03/bundlecheck")
	}
}

// TestDocumentationContract_CLICommandsAndFlags verifies that all commands and flags
// mentioned in public documentation exist in the Cobra command structure.
func TestDocumentationContract_CLICommandsAndFlags(t *testing.T) {
	root := cmd.NewRootCommand()
	subcommands := make(map[string]map[string]bool)

	for _, c := range root.Commands() {
		flags := make(map[string]bool)
		c.Flags().VisitAll(func(f *pflag.Flag) {
			flags["--"+f.Name] = true
			if f.Shorthand != "" {
				flags["-"+f.Shorthand] = true
			}
		})
		subcommands[c.Name()] = flags
	}

	// Common documented commands and their flags to assert against the CLI tree
	expectedSpecs := map[string][]string{
		"summary":   {"--stats", "-s", "--dist", "-d", "--project", "-p", "--entry", "-e", "--gzip", "--suggest", "--format", "-f", "--output", "-o", "--top", "--filter"},
		"check":     {"--stats", "-s", "--dist", "-d", "--project", "-p", "--entry", "-e", "--max-initial", "--max-lazy", "--max-total", "--baseline", "--max-initial-delta", "--max-total-delta", "--format", "-f", "--output", "-o", "--config", "-c"},
		"measure":   {"--stats", "-s", "--dist", "-d", "--project", "-p", "--entry", "-e", "--baseline", "-b", "--max-initial-delta", "--max-total-delta", "--format", "-f", "--output", "-o"},
		"baseline":  {},
		"why":       {"--stats", "-s", "--dist", "-d", "--project", "-p", "--entry", "-e", "--format", "-f", "--output", "-o"},
		"suggest":   {"--stats", "-s", "--dist", "-d", "--project", "-p", "--entry", "-e", "--gzip", "--format", "-f", "--output", "-o"},
		"inspect":   {"--stats", "-s", "--dist", "-d", "--project", "-p", "--format", "-f", "--output", "-o"},
		"compare":   {"--format", "-f", "--output", "-o"},
		"init":      {"--project", "-p", "--headroom", "--from-angular-budgets", "--write"},
		"workspace": {},
		"mcp":       {},
	}

	for cmdName, expectedFlags := range expectedSpecs {
		flagsMap, exists := subcommands[cmdName]
		if !exists {
			t.Errorf("command %q documented but not registered on root command", cmdName)
			continue
		}
		for _, flag := range expectedFlags {
			if !flagsMap[flag] {
				t.Errorf("command %q missing expected documented flag %q", cmdName, flag)
			}
		}
	}
}

// TestDocumentationContract_EntryDocumentation verifies that --entry / -e, the Action input 'entry',
// source and glob examples, and the TotalJS whole-browser invariant are documented across
// README.md, npm/bundlecheck/README.md, docs/index.html, and action.yml.
func TestDocumentationContract_EntryDocumentation(t *testing.T) {
	actionData, err := os.ReadFile("action.yml")
	if err != nil {
		t.Fatalf("failed to read action.yml: %v", err)
	}
	var actionDef struct {
		Inputs map[string]struct {
			Description string `yaml:"description"`
			Required    bool   `yaml:"required"`
			Default     string `yaml:"default"`
		} `yaml:"inputs"`
	}
	if err := yaml.Unmarshal(actionData, &actionDef); err != nil {
		t.Fatalf("failed to parse action.yml: %v", err)
	}
	entryInput, ok := actionDef.Inputs["entry"]
	if !ok {
		t.Errorf("action.yml missing input 'entry'")
	} else {
		if entryInput.Required {
			t.Errorf("action.yml input 'entry' should not be required")
		}
		if entryInput.Default != "" {
			t.Errorf("action.yml input 'entry' default should be empty string, got %q", entryInput.Default)
		}
		if !strings.Contains(strings.ToLower(entryInput.Description), "entrypoint") {
			t.Errorf("action.yml input 'entry' description should mention 'entrypoint', got %q", entryInput.Description)
		}
	}

	docFiles := []string{
		"README.md",
		filepath.Join("npm", "bundlecheck", "README.md"),
		filepath.Join("docs", "index.html"),
	}

	for _, docFile := range docFiles {
		contentBytes, err := os.ReadFile(docFile)
		if err != nil {
			t.Fatalf("failed to read %s: %v", docFile, err)
		}
		content := string(contentBytes)

		// 1. Must document --entry and -e
		if !strings.Contains(content, "--entry") {
			t.Errorf("%s missing '--entry' documentation", docFile)
		}
		if !strings.Contains(content, "-e") {
			t.Errorf("%s missing '-e' documentation", docFile)
		}

		// 2. Must document GitHub Action 'entry' input
		if !strings.Contains(content, "entry") || (!strings.Contains(content, "entry:") && !strings.Contains(content, "`entry`")) {
			t.Errorf("%s missing GitHub Action 'entry' input documentation", docFile)
		}

		// 3. Must document both source path and emitted chunk glob examples for entry
		hasSource := strings.Contains(content, "src/main.ts")
		hasGlob := strings.Contains(content, "main-*.js") || strings.Contains(content, "*.js") || strings.Contains(content, "*worker.js") || strings.Contains(content, "worker.js")
		if !hasSource {
			t.Errorf("%s missing source path entry example (e.g. src/main.ts)", docFile)
		}
		if !hasGlob {
			t.Errorf("%s missing emitted chunk glob entry example (e.g. main-*.js)", docFile)
		}

		// 4. Invariant: --entry scopes initial/lazy reachability and root traces, while TotalJS reflects the whole browser build
		lower := strings.ToLower(content)
		hasTotalInvariant := (strings.Contains(lower, "totaljs") || strings.Contains(lower, "total js")) &&
			(strings.Contains(lower, "whole") || strings.Contains(lower, "entire") || strings.Contains(lower, "all"))
		hasReachability := strings.Contains(lower, "reachab") || strings.Contains(lower, "initial") || strings.Contains(lower, "trace")
		if !hasTotalInvariant || !hasReachability {
			t.Errorf("%s missing explanation of invariant: --entry scopes initial/lazy reachability while TotalJS reflects whole browser build", docFile)
		}
	}
}
