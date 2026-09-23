package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundleradar/internal/report"
	"github.com/sonuKumar03/bundleradar/internal/workspace"
)

func workspaceCommand() *cobra.Command {
	parent := &cobra.Command{Use: "workspace", Short: "Analyze existing Angular builds across an Nx workspace"}
	var root, projects, target, configuration, format, output string
	var apps []string
	var top int
	var all, showGzip bool
	c := &cobra.Command{Use: "summary", Short: "Compare app sizes and shared npm/library contributions", Args: cobra.NoArgs, RunE: func(c *cobra.Command, args []string) error {
		if format != "text" && format != "json" && !report.IsMarkdownFormat(format) {
			return fmt.Errorf("unsupported format %q: use text, json, or markdown", format)
		}
		if top < 1 || target == "" || configuration == "" {
			return fmt.Errorf("--top must be positive and --target/--configuration must be nonempty")
		}
		var selected []string
		if c.Flags().Changed("projects") {
			for _, name := range strings.Split(projects, ",") {
				name = strings.TrimSpace(name)
				if name == "" {
					return fmt.Errorf("--projects requires comma-separated project names")
				}
				selected = append(selected, name)
			}
		}

		var appTargets []workspace.AppTarget
		if len(apps) > 0 {
			var err error
			appTargets, err = workspace.ParseAppTargets(root, nil, apps)
			if err != nil {
				return err
			}
		} else if len(selected) > 0 {
			if _, rootErr := workspace.Root(root, root != ""); rootErr != nil {
				var err error
				appTargets, err = workspace.ParseAppTargets(root, selected, nil)
				if err != nil {
					return fmt.Errorf("%w; (and no nx.json found: %v)", err, rootErr)
				}
			}
		}

		needCompression := format == "json" || showGzip || output != ""
		r, err := workspace.Analyze(c.Context(), workspace.AnalyzeOptions{
			Root:            root,
			ExplicitRoot:    root != "",
			Target:          target,
			Configuration:   configuration,
			Projects:        selected,
			AppTargets:      appTargets,
			WithCompression: needCompression,
			AllowNxFallback: true,
		})
		if err != nil {
			return err
		}
		w, cleanup, err := getOutputWriter(c, output)
		if err != nil {
			return err
		}
		if format == "json" {
			err = report.JSON(w, r)
		} else {
			err = report.Workspace(w, r, report.TextOptions{Top: top, All: all}, report.IsMarkdownFormat(format))
		}
		closeErr := cleanup()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if !r.Complete {
			hasEligible := false
			for _, app := range r.Apps {
				if app.Status != "unsupported" && app.Status != "unknown-project" {
					hasEligible = true
					break
				}
			}
			if !hasEligible {
				return fmt.Errorf("workspace report incomplete: no supported Angular apps selected")
			}
			return fmt.Errorf("workspace report incomplete: see app diagnostics")
		}
		return nil
	}}
	c.Flags().StringVar(&root, "root", "", "Nx workspace root (nearest ancestor by default)")
	c.Flags().StringVar(&projects, "projects", "", "Comma-separated app names (all supported applications by default)")
	c.Flags().StringVar(&target, "target", "build", "Angular build target")
	c.Flags().StringVar(&configuration, "configuration", "production", "Configuration used to resolve existing outputs")
	c.Flags().StringVarP(&format, "format", "f", "text", "Output format: text, json, or markdown")
	c.Flags().StringVarP(&output, "output", "o", "", "Write report to a file instead of stdout")
	c.Flags().IntVar(&top, "top", 10, "Number of npm/library contributors to display")
	c.Flags().BoolVar(&all, "all", false, "Display all contributors")
	c.Flags().BoolVarP(&showGzip, "gzip", "g", false, "Include estimated Gzip wire transfer sizes in report")
	c.Flags().StringSliceVar(&apps, "app", nil, "Explicit app target mapping name=stats_path[:dist_path] (can be repeated)")
	parent.AddCommand(c)
	return parent
}
