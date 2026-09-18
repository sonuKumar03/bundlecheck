// Package config loads and parses project-level .bundlecheck.yml configuration.
package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/budget"
)

type Config struct {
	Budgets ConfigBudgets `yaml:"budgets"`
	Rules   ConfigRules   `yaml:"rules"`
}

type ConfigBudgets struct {
	InitialJSMax     string `yaml:"initial_js_max"`
	LazyJSMax        string `yaml:"lazy_js_max"`
	TotalMax         string `yaml:"total_max"`
	MaxInitialDelta  string `yaml:"max_initial_delta"`
	MaxTotalDelta    string `yaml:"max_total_delta"`
	MaxDeltaIncrease string `yaml:"max_delta_increase"`
}

type ConfigRules struct {
	DisallowPackages []string `yaml:"disallow_packages"`
}

// FindAndLoad looks for .bundlecheck.yml or .bundlecheck.yaml in rootDir or parent directories.
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
		for _, name := range []string{".bundlecheck.yml", ".bundlecheck.yaml", ".bundlecheck/config.yml", ".bundlecheck/config.yaml"} {
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
