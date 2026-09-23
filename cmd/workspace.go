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
		needCompression := format == "json" || showGzip || output != ""
		r, err := workspace.Analyze(c.Context(), workspace.AnalyzeOptions{
			Root:            root,
			ExplicitRoot:    root != "",
			Target:          target,
			Configuration:   configuration,
			Projects:        selected,
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
	parent.AddCommand(c)
	return parent
}
