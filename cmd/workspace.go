package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"bundlecheck/internal/report"
	"bundlecheck/internal/snapshot"
	"bundlecheck/internal/workspace"
)

func workspaceCommand() *cobra.Command {
	parent := &cobra.Command{Use: "workspace", Short: "Analyze existing Angular builds across an Nx workspace"}
	var root, projects, target, configuration, format, output string
	var top int
	var all bool
	c := &cobra.Command{Use: "summary", Short: "Compare app sizes and shared npm/library contributions", Args: cobra.NoArgs, RunE: func(c *cobra.Command, args []string) error {
		if format != "text" && format != "json" && !report.IsMarkdownFormat(format) {
			return fmt.Errorf("unsupported format %q: use text, json, or markdown", format)
		}
		if top < 1 || target == "" || configuration == "" {
			return fmt.Errorf("--top must be positive and --target/--configuration must be nonempty")
		}
		explicitRoot := root != ""
		start := root
		if !explicitRoot {
			var err error
			start, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		workspaceRoot, err := workspace.Root(start, explicitRoot)
		if err != nil {
			return err
		}
		metadata, err := workspace.ReadMetadata(c.Context(), workspaceRoot)
		if err != nil {
			return err
		}
		r := workspace.NewResult(workspaceRoot, target, configuration)
		selected := []string{}
		explicitProjects := c.Flags().Changed("projects")
		if explicitProjects {
			for _, name := range strings.Split(projects, ",") {
				name = strings.TrimSpace(name)
				if name == "" {
					return fmt.Errorf("--projects requires comma-separated project names")
				}
				selected = append(selected, name)
			}
		} else {
			for name, p := range metadata.Graph.Nodes {
				if workspace.IsApplication(p) {
					selected = append(selected, name)
				}
			}
		}
		slices.Sort(selected)
		selected = slices.Compact(selected)
		packages := map[string][]snapshot.Package{}
		libraries := map[string][]snapshot.Package{}
		eligible := 0
		for _, name := range selected {
			app := workspace.App{Name: name}
			p, exists := metadata.Graph.Nodes[name]
			switch {
			case !exists:
				app.Status = "unknown-project"
				app.Diagnostic = "project not present in Nx graph"
				r.Complete = false
			case !workspace.IsApplication(p) || !workspace.Supported(p, target):
				app.Status = "unsupported"
				app.Diagnostic = fmt.Sprintf("target %s executor %q is not a supported Angular application esbuild builder", target, p.Data.Targets[target].Executor)
				if explicitProjects {
					r.Complete = false
				}
			default:
				eligible++
				base, browser, resolveErr := workspace.OutputDirectories(workspaceRoot, p, target, configuration)
				if resolveErr != nil {
					app.Status = "configuration-error"
					app.Diagnostic = resolveErr.Error()
					r.Complete = false
					break
				}
				app.Stats, app.Dist, resolveErr = workspace.Artifacts(base, browser)
				if resolveErr != nil {
					app.Status = "missing-artifacts"
					app.Diagnostic = resolveErr.Error()
					r.Complete = false
					break
				}
				app.Status = "ready"
			}
			r.Apps = append(r.Apps, app)
		}
		// Reject artifacts claimed by multiple selected projects before analyzing either.
		owners := map[string][]int{}
		for i, app := range r.Apps {
			if app.Status == "ready" {
				for _, p := range []string{app.Stats, app.Dist} {
					canonical, err := filepath.EvalSymlinks(p)
					if err != nil {
						canonical = p
					}
					owners[canonical] = append(owners[canonical], i)
				}
			}
		}
		for _, indexes := range owners {
			if len(indexes) > 1 {
				for _, i := range indexes {
					r.Apps[i].Status = "ambiguous-artifacts"
					r.Apps[i].Diagnostic = "artifact location is claimed by multiple selected projects"
					r.Complete = false
				}
			}
		}
		for i := range r.Apps {
			app := &r.Apps[i]
			if app.Status != "ready" {
				continue
			}
			needCompression := format == "json"
			result, snap, analysisErr := runAnalysisWithOptions(app.Stats, app.Dist, needCompression)
			if analysisErr == nil {
				libraries[app.Name], analysisErr = workspace.Libraries(workspaceRoot, metadata.Graph.Nodes, snap)
			}
			if analysisErr != nil {
				app.Status = "analysis-error"
				app.Diagnostic = analysisErr.Error()
				r.Complete = false
				continue
			}
			app.Status = "analyzed"
			app.Analysis = result
			packages[app.Name] = result.Packages
			app.DrillDown = [][]string{{"bundlecheck", "why", "--stats", app.Stats, "--dist", app.Dist, "--package", "<package-name>"}, {"bundlecheck", "suggest", "--stats", app.Stats, "--dist", app.Dist}}
		}
		if eligible == 0 {
			r.Complete = false
		}
		r.Packages, err = workspace.Matrix(r.Apps, packages)
		if err != nil {
			return err
		}
		r.Libraries, err = workspace.Matrix(r.Apps, libraries)
		if err != nil {
			return err
		}
		r.Findings = workspace.Findings(r.Packages, r.Libraries)
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
			if eligible == 0 {
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
	parent.AddCommand(c)
	return parent
}
