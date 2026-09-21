package main_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const skillDir = ".agents/skills/bundlecheck"

type skillMetadata struct {
	Name          string `yaml:"name"`
	Description   string `yaml:"description"`
	Compatibility string `yaml:"compatibility"`
}

func readSkill(t *testing.T) (skillMetadata, string) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(string(data), "---", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[0]) != "" {
		t.Fatal("SKILL.md must start with YAML frontmatter")
	}

	var meta skillMetadata
	if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
		t.Fatalf("parse SKILL.md frontmatter: %v", err)
	}
	return meta, parts[2]
}

func TestAgentSkillMetadata(t *testing.T) {
	meta, _ := readSkill(t)
	if meta.Name != "bundlecheck" {
		t.Fatalf("name = %q, want bundlecheck", meta.Name)
	}
	for _, term := range []string{"Angular", "esbuild", "bundle size", "Do not use"} {
		if !strings.Contains(meta.Description, term) {
			t.Errorf("description must contain %q", term)
		}
	}
	for _, term := range []string{"bundlecheck CLI", "stats.json"} {
		if !strings.Contains(meta.Compatibility, term) {
			t.Errorf("compatibility must contain %q", term)
		}
	}
}

func TestAgentSkillProgressiveDisclosure(t *testing.T) {
	_, body := readSkill(t)
	if lines := strings.Count(body, "\n") + 1; lines > 120 {
		t.Errorf("SKILL.md body has %d lines, want at most 120", lines)
	}
	if strings.Contains(body, "./install.sh --with-skill") {
		t.Fatal("skill must not assume the bundlecheck source checkout")
	}

	targets := regexp.MustCompile(`references/[a-z0-9-]+\.md`).FindAllString(body, -1)
	unique := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		unique[target] = struct{}{}
	}
	got := make([]string, 0, len(unique))
	for target := range unique {
		got = append(got, target)
	}
	sort.Strings(got)
	want := []string{"references/ci.md", "references/cli.md", "references/json-schema.md", "references/nx.md"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("references = %v, want %v", got, want)
	}
	for _, target := range got {
		if _, err := os.Stat(filepath.Join(skillDir, target)); err != nil {
			t.Errorf("reference %s: %v", target, err)
		}
	}
}
