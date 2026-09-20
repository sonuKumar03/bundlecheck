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

	yamlBlockRegex := regexp.MustCompile("(?s)```ya?ml\\s*\n(# \\.bundlecheck\\.yml.*?)```")

	for _, docFile := range docFiles {
		content, err := os.ReadFile(docFile)
		if err != nil {
			t.Fatalf("failed to read %s: %v", docFile, err)
		}

		matches := yamlBlockRegex.FindAllStringSubmatch(string(content), -1)
		for _, match := range matches {
			snippet := match[1]
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
		"summary":   {"--stats", "-s", "--dist", "-d", "--project", "-p", "--gzip", "--suggest", "--format", "-f", "--output", "-o", "--top", "--filter"},
		"check":     {"--stats", "-s", "--dist", "-d", "--project", "-p", "--max-initial", "--max-lazy", "--max-total", "--baseline", "--max-initial-delta", "--max-total-delta", "--format", "-f", "--output", "-o", "--config", "-c"},
		"measure":   {"--stats", "-s", "--dist", "-d", "--project", "-p", "--baseline", "-b", "--max-initial-delta", "--max-total-delta", "--format", "-f", "--output", "-o"},
		"baseline":  {},
		"why":       {"--stats", "-s", "--dist", "-d", "--project", "-p", "--format", "-f", "--output", "-o"},
		"suggest":   {"--stats", "-s", "--dist", "-d", "--project", "-p", "--gzip", "--format", "-f", "--output", "-o"},
		"inspect":   {"--stats", "-s", "--dist", "-d", "--project", "-p", "--format", "-f", "--output", "-o"},
		"compare":   {"--format", "-f", "--output", "-o"},
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
