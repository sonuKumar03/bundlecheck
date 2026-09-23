package cmd

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/sonuKumar03/bundleradar/internal/budget"
	"github.com/sonuKumar03/bundleradar/internal/config"
)

func initCommand() *cobra.Command {
	var (
		project            string
		headroom           float64
		fromAngularBudgets bool
		write              bool
	)

	c := &cobra.Command{
		Use:   "init [path]",
		Short: "Generate or propose a .bundleradar.yml configuration",
		Long: `Generate a starting .bundleradar.yml budget configuration from measured bundle size or existing Angular budgets.
Without --write, the command prints the proposed configuration and its derivation to stdout.
With --write, it writes .bundleradar.yml only if the destination does not already exist.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			targetDir := "."
			if len(args) == 1 {
				targetDir = args[0]
			}
			absTarget, err := filepath.Abs(targetDir)
			if err != nil {
				return err
			}

			var cfg *config.Config
			var derivation string

			if fromAngularBudgets {
				// Look for angular.json in targetDir or parent
				angularJSONPath := filepath.Join(absTarget, "angular.json")
				if _, err := os.Stat(angularJSONPath); os.IsNotExist(err) {
					// Also check project.json
					projJSONPath := filepath.Join(absTarget, "project.json")
					if _, err := os.Stat(projJSONPath); os.IsNotExist(err) {
						return &UsageError{Err: fmt.Errorf("neither angular.json nor project.json found in %q for --from-angular-budgets", absTarget)}
					}
					angularJSONPath = projJSONPath
				}

				cfg, err = config.LoadFromAngularJSON(angularJSONPath, project)
				if err != nil {
					return &UsageError{Err: fmt.Errorf("import angular budgets: %w", err)}
				}
				derivation = fmt.Sprintf("# Imported from Angular budgets in %s\n", angularJSONPath)
			} else {
				// Measure current build
				sFile, dDir, err := resolveBuildArtifactsInDir(absTarget, "", "", project)
				if err != nil {
					return err
				}

				res, err := runAnalysis(sFile, dDir)
				if err != nil {
					return err
				}

				if headroom < 0 {
					return &UsageError{Err: fmt.Errorf("--headroom must be nonnegative, got %.2f", headroom)}
				}

				measuredInitial := res.Summary.InitialJS
				measuredTotal := res.Summary.TotalJS

				limitInitial := int64(math.Ceil(float64(measuredInitial) * (1.0 + headroom/100.0)))
				limitTotal := int64(math.Ceil(float64(measuredTotal) * (1.0 + headroom/100.0)))

				cfg = &config.Config{
					Budgets: config.ConfigBudgets{
						InitialJSMax: budget.FormatBytes(limitInitial),
						TotalMax:     budget.FormatBytes(limitTotal),
					},
				}

				derivation = fmt.Sprintf("# Proposed configuration with %.1f%% headroom:\n# Measured: Initial JS %s (%d bytes), Total JS %s (%d bytes)\n# Proposed: initial_js_max: %s, total_max: %s\n",
					headroom,
					budget.FormatBytes(measuredInitial), measuredInitial,
					budget.FormatBytes(measuredTotal), measuredTotal,
					cfg.Budgets.InitialJSMax, cfg.Budgets.TotalMax,
				)
			}

			// Render YAML
			yamlBytes, err := config.RenderYAML(cfg)
			if err != nil {
				return err
			}

			// Strict validate generated YAML
			strictDec := yaml.NewDecoder(bytes.NewReader(yamlBytes))
			strictDec.KnownFields(true)
			var testCfg config.Config
			if err := strictDec.Decode(&testCfg); err != nil {
				return fmt.Errorf("internal error: rendered invalid YAML: %w", err)
			}

			destPath := filepath.Join(absTarget, ".bundleradar.yml")
			destAlt := filepath.Join(absTarget, ".bundleradar.yaml")

			if write {
				if _, err := os.Stat(destPath); err == nil {
					return &UsageError{Err: fmt.Errorf("configuration file %q already exists; refusing to overwrite", destPath)}
				}
				if _, err := os.Stat(destAlt); err == nil {
					return &UsageError{Err: fmt.Errorf("configuration file %q already exists; refusing to overwrite", destAlt)}
				}

				content := []byte(derivation + "\n" + string(yamlBytes))
				if err := os.WriteFile(destPath, content, 0644); err != nil {
					return fmt.Errorf("write %q: %w", destPath, err)
				}
				fmt.Fprintf(c.OutOrStdout(), "Created configuration at %s\n", destPath)
				return nil
			}

			// Preview mode
			fmt.Fprint(c.OutOrStdout(), derivation)
			fmt.Fprint(c.OutOrStdout(), "\n")
			fmt.Fprint(c.OutOrStdout(), string(yamlBytes))
			return nil
		},
	}

	c.Flags().StringVarP(&project, "project", "p", "", "Project name for multi-project workspaces")
	c.Flags().Float64Var(&headroom, "headroom", 5.0, "Headroom percentage to add over measured bundle sizes")
	c.Flags().BoolVar(&fromAngularBudgets, "from-angular-budgets", false, "Import existing budget limits from angular.json or project.json")
	c.Flags().BoolVar(&write, "write", false, "Write proposed configuration to .bundleradar.yml if file does not exist")

	return c
}
