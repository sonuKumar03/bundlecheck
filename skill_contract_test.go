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

const skillDir = ".agents/skills/bundleradar"

type skillMetadata struct {
	Name          string `yaml:"name"`
	Description   string `yaml:"description"`
	Compatibility string `yaml:"compatibility"`
}

type triggerEval struct {
	ID            string `yaml:"id"`
	Prompt        string `yaml:"prompt"`
	ShouldTrigger bool   `yaml:"should_trigger"`
}

type workflowEval struct {
	ID              string   `yaml:"id"`
	Prompt          string   `yaml:"prompt"`
	RequiredStages  []string `yaml:"required_stages"`
	ForbiddenClaims []string `yaml:"forbidden_claims"`
}

func readEval[T any](t *testing.T, name string) []T {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "skill-evals", name))
	if err != nil {
		t.Fatal(err)
	}
	var cases []T
	if err := yaml.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return cases
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
	if meta.Name != "bundleradar" {
		t.Fatalf("name = %q, want bundleradar", meta.Name)
	}
	for _, term := range []string{"Angular", "esbuild", "bundle size", "Do not use"} {
		if !strings.Contains(meta.Description, term) {
			t.Errorf("description must contain %q", term)
		}
	}
	for _, term := range []string{"bundleradar CLI", "stats.json"} {
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
		t.Fatal("skill must not assume the bundleradar source checkout")
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

func TestSkillEvalTriggers(t *testing.T) {
	cases := readEval[triggerEval](t, "triggers.yaml")
	seen := make(map[string]bool, len(cases))
	positive := 0
	for _, tc := range cases {
		if tc.ID == "" || tc.Prompt == "" {
			t.Error("trigger IDs and prompts must be non-empty")
		}
		if seen[tc.ID] {
			t.Errorf("duplicate trigger ID %q", tc.ID)
		}
		seen[tc.ID] = true
		if tc.ShouldTrigger {
			positive++
		}
	}
	if positive != 10 || len(cases)-positive != 10 {
		t.Errorf("trigger counts = %d positive, %d negative; want 10 each", positive, len(cases)-positive)
	}
}

func TestSkillEvalWorkflows(t *testing.T) {
	cases := readEval[workflowEval](t, "workflows.yaml")
	seen := make(map[string]workflowEval, len(cases))
	for _, tc := range cases {
		if tc.ID == "" || tc.Prompt == "" {
			t.Error("workflow IDs and prompts must be non-empty")
		}
		if _, ok := seen[tc.ID]; ok {
			t.Errorf("duplicate workflow ID %q", tc.ID)
		}
		if len(tc.ForbiddenClaims) == 0 {
			t.Errorf("workflow %q has no forbidden claims", tc.ID)
		}
		seen[tc.ID] = tc
	}
	for _, id := range []string{"diagnose", "optimize", "pr-regression", "ci-gate", "nx-comparison"} {
		if _, ok := seen[id]; !ok {
			t.Errorf("missing workflow %q", id)
		}
	}
	optimize := seen["optimize"]
	stages := make(map[string]bool, len(optimize.RequiredStages))
	for _, stage := range optimize.RequiredStages {
		stages[stage] = true
	}
	for _, stage := range []string{"baseline", "summary-or-suggest", "why", "source-inspection", "authorized-edit", "production-rebuild", "project-tests", "measure", "report-signed-delta"} {
		if !stages[stage] {
			t.Errorf("optimize workflow missing stage %q", stage)
		}
	}
}
