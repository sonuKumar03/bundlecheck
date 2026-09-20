package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/build"
	"bundlecheck/internal/compression"
	"bundlecheck/internal/snapshot"
)

// AnalyzeOptions provides configuration for workspace analysis.
type AnalyzeOptions struct {
	Root            string
	ExplicitRoot    bool
	Target          string
	Configuration   string
	Projects        []string
	WithCompression bool
}

// Analyze runs comprehensive workspace inspection across Angular apps in an Nx workspace.
func Analyze(ctx context.Context, opts AnalyzeOptions) (*Result, error) {
	if opts.Target == "" {
		opts.Target = "build"
	}
	if opts.Configuration == "" {
		opts.Configuration = "production"
	}

	start := opts.Root
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	workspaceRoot, err := Root(start, opts.ExplicitRoot)
	if err != nil {
		return nil, err
	}

	metadata, err := ReadMetadata(ctx, workspaceRoot)
	if err != nil {
		return nil, err
	}

	r := NewResult(workspaceRoot, opts.Target, opts.Configuration)

	selected := []string{}
	explicitProjects := len(opts.Projects) > 0
	if explicitProjects {
		selected = append(selected, opts.Projects...)
	} else {
		for name, p := range metadata.Graph.Nodes {
			if IsApplication(p) {
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
		app := App{Name: name}
		p, exists := metadata.Graph.Nodes[name]
		switch {
		case !exists:
			app.Status = "unknown-project"
			app.Diagnostic = "project not present in Nx graph"
			r.Complete = false
		case !IsApplication(p) || !Supported(p, opts.Target):
			app.Status = "unsupported"
			app.Diagnostic = fmt.Sprintf("target %s executor %q is not a supported Angular application esbuild builder", opts.Target, p.Data.Targets[opts.Target].Executor)
			if explicitProjects {
				r.Complete = false
			}
		default:
			eligible++
			base, browser, resolveErr := OutputDirectories(workspaceRoot, p, opts.Target, opts.Configuration)
			if resolveErr != nil {
				app.Status = "configuration-error"
				app.Diagnostic = resolveErr.Error()
				r.Complete = false
				break
			}
			app.Stats, app.Dist, resolveErr = Artifacts(base, browser)
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

		snap, loadErr := build.Load(app.Stats, app.Dist)
		if loadErr != nil {
			app.Status = "analysis-error"
			app.Diagnostic = loadErr.Error()
			r.Complete = false
			continue
		}

		result, analysisErr := analysis.Analyze(snap)
		if analysisErr == nil && opts.WithCompression {
			compression.AttachCompression(snap, app.Dist)
			result.Summary = snap.Totals
			result.Packages = snap.Packages
		}

		if analysisErr == nil {
			libraries[app.Name], analysisErr = Libraries(workspaceRoot, metadata.Graph.Nodes, snap)
		}
		if analysisErr != nil {
			app.Status = "analysis-error"
			app.Diagnostic = analysisErr.Error()
			r.Complete = false
			continue
		}

		app.Status = "analyzed"
		app.Analysis = result
		app.Freshness = CheckFreshness(workspaceRoot, metadata.Graph.Nodes[app.Name].Data.Root, app.Stats, snap)
		packages[app.Name] = result.Packages
		app.DrillDown = [][]string{
			{"bundlecheck", "why", "--stats", app.Stats, "--dist", app.Dist, "--package", "<package-name>"},
			{"bundlecheck", "suggest", "--stats", app.Stats, "--dist", app.Dist},
		}
	}

	if eligible == 0 {
		r.Complete = false
	}

	var errMatrix error
	r.Packages, errMatrix = Matrix(r.Apps, packages)
	if errMatrix != nil {
		return nil, errMatrix
	}

	r.Libraries, errMatrix = Matrix(r.Apps, libraries)
	if errMatrix != nil {
		return nil, errMatrix
	}

	r.Findings = Findings(r.Packages, r.Libraries)

	return r, nil
}
