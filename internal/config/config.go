// Package config loads and parses project-level .bundleradar.yml configuration.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sonuKumar03/bundleradar/internal/analysis"
	"github.com/sonuKumar03/bundleradar/internal/budget"
)

type Config struct {
	Budgets ConfigBudgets `yaml:"budgets,omitempty"`
	Rules   ConfigRules   `yaml:"rules,omitempty"`
}

type ConfigBudgets struct {
	InitialJSMax     string `yaml:"initial_js_max,omitempty"`
	LazyJSMax        string `yaml:"lazy_js_max,omitempty"`
	TotalMax         string `yaml:"total_max,omitempty"`
	MaxInitialDelta  string `yaml:"max_initial_delta,omitempty"`
	MaxTotalDelta    string `yaml:"max_total_delta,omitempty"`
	MaxDeltaIncrease string `yaml:"max_delta_increase,omitempty"`
}

type ConfigRules struct {
	DisallowPackages []string `yaml:"disallow_packages,omitempty"`
}

// RenderYAML serializes a Config struct into clean YAML bytes.
func RenderYAML(cfg *Config) ([]byte, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cannot render nil config")
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		return nil, fmt.Errorf("encode config YAML: %w", err)
	}
	return buf.Bytes(), nil
}

// AngularBudget represents a budget entry in angular.json or project.json.
type AngularBudget struct {
	Type           string `json:"type"`
	Name           string `json:"name,omitempty"`
	MaximumWarning string `json:"maximumWarning,omitempty"`
	MaximumError   string `json:"maximumError,omitempty"`
}

// LoadFromAngularJSON extracts bundle budgets from an angular.json or project.json file.
func LoadFromAngularJSON(jsonPath string, projectName string) (*Config, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", jsonPath, err)
	}

	var root struct {
		Projects map[string]struct {
			Architect map[string]struct {
				Configurations map[string]struct {
					Budgets []AngularBudget `json:"budgets"`
				} `json:"configurations"`
			} `json:"architect"`
			Targets map[string]struct {
				Configurations map[string]struct {
					Budgets []AngularBudget `json:"budgets"`
				} `json:"configurations"`
			} `json:"targets"`
		} `json:"projects"`
		Targets map[string]struct {
			Configurations map[string]struct {
				Budgets []AngularBudget `json:"budgets"`
			} `json:"configurations"`
		} `json:"targets"`
	}

	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse angular/project JSON: %w", err)
	}

	var budgets []AngularBudget
	if len(root.Projects) > 0 {
		var proj struct {
			Architect map[string]struct {
				Configurations map[string]struct {
					Budgets []AngularBudget `json:"budgets"`
				} `json:"configurations"`
			} `json:"architect"`
			Targets map[string]struct {
				Configurations map[string]struct {
					Budgets []AngularBudget `json:"budgets"`
				} `json:"configurations"`
			} `json:"targets"`
		}
		if projectName != "" {
			var ok bool
			proj, ok = root.Projects[projectName]
			if !ok {
				return nil, fmt.Errorf("project %q not found in %q", projectName, jsonPath)
			}
		} else {
			for _, p := range root.Projects {
				proj = p
				break
			}
		}
		if buildTarget, ok := proj.Architect["build"]; ok {
			if prod, ok := buildTarget.Configurations["production"]; ok {
				budgets = prod.Budgets
			}
		}
		if len(budgets) == 0 {
			if buildTarget, ok := proj.Targets["build"]; ok {
				if prod, ok := buildTarget.Configurations["production"]; ok {
					budgets = prod.Budgets
				}
			}
		}
	} else if len(root.Targets) > 0 {
		if buildTarget, ok := root.Targets["build"]; ok {
			if prod, ok := buildTarget.Configurations["production"]; ok {
				budgets = prod.Budgets
			}
		}
	}

	if len(budgets) == 0 {
		return nil, fmt.Errorf("no production build budgets found in %q", jsonPath)
	}

	cfg := &Config{}
	for _, b := range budgets {
		limitStr := b.MaximumError
		if limitStr == "" {
			limitStr = b.MaximumWarning
		}
		if limitStr == "" {
			continue
		}
		val, err := budget.ParseBytes(limitStr)
		if err != nil {
			continue
		}
		formatted := budget.FormatBytes(val)

		switch strings.ToLower(b.Type) {
		case "initial", "allscript":
			cfg.Budgets.InitialJSMax = formatted
		case "bundle":
			if strings.EqualFold(b.Name, "main") {
				if cfg.Budgets.InitialJSMax == "" {
					cfg.Budgets.InitialJSMax = formatted
				}
			} else if cfg.Budgets.TotalMax == "" {
				cfg.Budgets.TotalMax = formatted
			}
		case "all", "total":
			cfg.Budgets.TotalMax = formatted
		}
	}

	return cfg, nil
}

// FindAndLoad looks for .bundleradar.yml or .bundleradar.yaml in rootDir or parent directories.
func FindAndLoad(rootDir string) (*Config, string, error) {
	if rootDir == "" {
		var err error
		rootDir, err = os.Getwd()
		if err != nil {
			return nil, "", err
		}
	}

	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, "", err
	}
	for {
		for _, name := range []string{".bundleradar.yml", ".bundleradar.yaml", ".bundleradar/config.yml", ".bundleradar/config.yaml"} {
			candidate := filepath.Join(rootDir, name)
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				continue
			} else if err != nil {
				return nil, candidate, err
			}
			cfg, err := LoadFile(candidate)
			return cfg, candidate, err
		}
		parent := filepath.Dir(rootDir)
		if parent == rootDir {
			return nil, "", nil
		}
		rootDir = parent
	}
}

// LoadFile reads and unmarshals a YAML configuration file.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config YAML %q: %w", path, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("config %q must contain a single YAML document", path)
	}

	return &cfg, nil
}

// ApplyToLimits populates unset CLI limits from config budgets.
func (c *Config) ApplyToLimits(limits *budget.Limits) error {
	if c == nil || limits == nil {
		return nil
	}

	if limits.MaxInitial == nil && c.Budgets.InitialJSMax != "" {
		val, err := budget.ParseBytes(c.Budgets.InitialJSMax)
		if err != nil {
			return fmt.Errorf("config initial_js_max: %w", err)
		}
		limits.MaxInitial = &val
	}

	if limits.MaxLazy == nil && c.Budgets.LazyJSMax != "" {
		val, err := budget.ParseBytes(c.Budgets.LazyJSMax)
		if err != nil {
			return fmt.Errorf("config lazy_js_max: %w", err)
		}
		limits.MaxLazy = &val
	}

	if limits.MaxTotal == nil && c.Budgets.TotalMax != "" {
		val, err := budget.ParseBytes(c.Budgets.TotalMax)
		if err != nil {
			return fmt.Errorf("config total_max: %w", err)
		}
		limits.MaxTotal = &val
	}

	if limits.MaxInitialDelta == nil && c.Budgets.MaxInitialDelta != "" {
		val, err := budget.ParseBytes(c.Budgets.MaxInitialDelta)
		if err != nil {
			return fmt.Errorf("config max_initial_delta: %w", err)
		}
		limits.MaxInitialDelta = &val
	}

	if limits.MaxTotalDelta == nil {
		deltaStr := c.Budgets.MaxTotalDelta
		if deltaStr == "" {
			deltaStr = c.Budgets.MaxDeltaIncrease
		}
		if deltaStr != "" {
			val, err := budget.ParseBytes(deltaStr)
			if err != nil {
				return fmt.Errorf("config max_total_delta: %w", err)
			}
			limits.MaxTotalDelta = &val
		}
	}

	return nil
}

// CheckRules evaluates disallowed packages and package rules against an AnalysisResult.
func (c *Config) CheckRules(res *analysis.AnalysisResult) []budget.Violation {
	if c == nil || res == nil || len(c.Rules.DisallowPackages) == 0 {
		return nil
	}

	var violations []budget.Violation
	disallowed := make(map[string]bool)
	for _, pkg := range c.Rules.DisallowPackages {
		disallowed[strings.ToLower(strings.TrimSpace(pkg))] = true
	}

	for _, p := range res.Packages {
		if disallowed[strings.ToLower(p.Name)] && p.TotalBytes > 0 {
			violations = append(violations, budget.Violation{
				Metric:  fmt.Sprintf("Disallowed package %q", p.Name),
				Actual:  p.TotalBytes,
				Limit:   0,
				Message: fmt.Sprintf("disallowed package %q found in bundle (%d bytes)", p.Name, p.TotalBytes),
			})
		}
	}

	return violations
}
