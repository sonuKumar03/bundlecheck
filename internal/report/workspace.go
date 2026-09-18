package report

import (
	"cmp"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"bundlecheck/internal/workspace"
)

// Workspace renders deployment costs without changing the machine-readable result.
func Workspace(w io.Writer, r *workspace.Result, opts TextOptions, markdown bool) error {
	var b strings.Builder
	escape := func(s string) string {
		if markdown {
			return strings.NewReplacer("|", "\\|", "\n", " ", "\r", " ", "`", "\\`").Replace(s)
		}
		return s
	}
	emphasis := func(s string) string {
		s = escape(s)
		if markdown {
			return "**" + s + "**"
		}
		return s
	}
	heading := func(title string) {
		if markdown {
			b.WriteString("### ")
		}
		b.WriteString(title + "\n\n")
	}
	row := func(values ...string) {
		for i := range values {
			values[i] = escape(values[i])
		}
		if markdown {
			b.WriteString("| " + strings.Join(values, " | ") + " |\n")
		} else {
			b.WriteString(strings.Join(values, "\t") + "\n")
		}
	}
	table := func(columns ...string) {
		row(columns...)
		if markdown {
			separators := make([]string, len(columns))
			for i := range separators {
				separators[i] = "---"
			}
			row(separators...)
		}
	}
	limit := func(n int) int {
		if opts.All {
			return n
		}
		top := opts.Top
		if top <= 0 {
			top = 10
		}
		if top < n {
			return top
		}
		return n
	}
	state := "Complete"
	if !r.Complete {
		state = "Partial — failed apps are unavailable; sums cover analyzed apps only"
	}
	if markdown {
		b.WriteString("## ")
	}
	fmt.Fprintf(&b, "Nx Workspace Bundle Summary\n\n%s · %s / %s\n\n", state, escape(r.Target), escape(r.Configuration))
	heading("Key findings")
	var largest *workspace.App
	for i := range r.Apps {
		app := &r.Apps[i]
		if app.Analysis != nil && (largest == nil || app.Analysis.Summary.InitialJS > largest.Analysis.Summary.InitialJS) {
			largest = app
		}
	}
	if largest != nil {
		fmt.Fprintf(&b, "- Largest startup bundle: %s, %s initial JS and %s lazy JS.\n", emphasis(largest.Name), workspaceSize(largest.Analysis.Summary.InitialJS), workspaceSize(largest.Analysis.Summary.LazyJS))
	}
	candidates := 0
	for _, p := range r.Packages {
		if p.InitialBytes == 0 || frameworkPackage(p.Name) {
			continue
		}
		details := []string{}
		for _, app := range r.Apps {
			if value := p.Apps[app.Name]; value != nil && value.InitialBytes > 0 {
				details = append(details, fmt.Sprintf("%s in %s (%s of startup)", workspaceSize(value.InitialBytes), escape(app.Name), workspaceShare(value.InitialBytes, app)))
			}
		}
		if len(details) > 0 {
			fmt.Fprintf(&b, "- %s: %s. Investigate its eager import paths.\n", emphasis(p.Name), strings.Join(details, "; "))
			candidates++
		}
		if candidates >= 3 {
			break
		}
	}
	if candidates == 0 {
		b.WriteString("- No additional third-party startup contributors measured.\n")
	}
	b.WriteString("\nThese are measured costs, not guaranteed savings. Repeated usage across apps does not imply a duplicate download or automatic deduplication.\n\n")
	heading("App overview")
	table("App", "Status", "Initial JS", "Lazy JS", "Total JS")
	for _, app := range r.Apps {
		initial, lazy, total := "unavailable", "unavailable", "unavailable"
		if app.Analysis != nil {
			initial = workspaceSize(app.Analysis.Summary.InitialJS)
			lazy = workspaceSize(app.Analysis.Summary.LazyJS)
			total = workspaceSize(app.Analysis.Summary.TotalJS)
		}
		row(app.Name, app.Status, initial, lazy, total)
	}
	for _, app := range r.Apps {
		if app.Diagnostic != "" {
			fmt.Fprintf(&b, "\n- %s: %s\n", escape(app.Name), escape(app.Diagnostic))
		}
	}
	b.WriteString("\nSizes use binary units (1 KiB = 1,024 bytes). Existing artifact freshness and build configuration are not verified.\n\n")
	// Initial and lazy contributions have separate matrices instead of packed cells.
	matrix := func(rows []workspace.Contributor, lazy bool) {
		columns := []string{"Package", "Sum across apps"}
		apps := []workspace.App{}
		for _, app := range r.Apps {
			if app.Status != "unsupported" {
				apps = append(apps, app)
				name := app.Name
				if !lazy {
					name += " initial (% of startup)"
				}
				columns = append(columns, name)
			}
		}
		table(columns...)
		for _, p := range rows {
			sum := p.InitialBytes
			if lazy {
				sum = p.LazyBytes
			}
			values := []string{p.Name, workspaceSize(sum)}
			for _, app := range apps {
				value := "unavailable"
				if size := p.Apps[app.Name]; size != nil {
					n := size.InitialBytes
					if lazy {
						n = size.LazyBytes
					}
					value = "—"
					if n > 0 {
						value = workspaceSize(n)
						if !lazy {
							value += " (" + workspaceShare(n, app) + ")"
						}
					}
				}
				values = append(values, value)
			}
			row(values...)
		}
	}
	heading("Startup contributors")
	b.WriteString("Ranked by initial JS summed across separate deployments. — means no contribution in an analyzed app.\n\n")
	initial := []workspace.Contributor{}
	lazy := []workspace.Contributor{}
	for _, p := range r.Packages {
		if p.InitialBytes > 0 {
			initial = append(initial, p)
		}
		if p.LazyBytes > 0 {
			lazy = append(lazy, p)
		}
	}
	initialShown := initial[:limit(len(initial))]
	for i := 0; i < 2; i++ {
		title := "Application dependencies"
		if i == 1 {
			title = "Framework/runtime — context, not automatic optimization targets"
		}
		rows := []workspace.Contributor{}
		for _, p := range initialShown {
			if frameworkPackage(p.Name) == (i == 1) {
				rows = append(rows, p)
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s\n\n", title)
		matrix(rows, false)
		b.WriteString("\n")
	}
	if len(initial) == 0 {
		b.WriteString("No initial package contributions measured.\n\n")
	}
	if len(initialShown) < len(initial) {
		fmt.Fprintf(&b, "Showing %d of %d initial packages; use --all for the full list.\n\n", len(initialShown), len(initial))
	}
	if len(lazy) > 0 {
		heading("Lazy contributors")
		// Initial ranking does not necessarily rank lazy costs.
		sorted := append([]workspace.Contributor(nil), lazy...)
		slices.SortFunc(sorted, func(a, b workspace.Contributor) int {
			if n := cmp.Compare(b.LazyBytes, a.LazyBytes); n != 0 {
				return n
			}
			return cmp.Compare(a.Name, b.Name)
		})
		matrix(sorted[:limit(len(sorted))], true)
		b.WriteString("\n")
	}
	heading("Library dependency context")
	b.WriteString("Own code excludes imported npm packages. These sizes are not dependency-inclusive library footprints; npm costs appear above. Import paths must be traced before assigning a package's cost to a library, and shared costs must not be added twice.\n\n")
	table("Library", "App", "Own initial", "Own lazy", "Own total")
	libraries := r.Libraries[:limit(len(r.Libraries))]
	for _, library := range libraries {
		for _, app := range r.Apps {
			size, exists := library.Apps[app.Name]
			if !exists {
				continue
			}
			if size == nil {
				row(library.Name, app.Name, "unavailable", "unavailable", "unavailable")
				continue
			}
			if size.TotalBytes == 0 {
				continue
			}
			row(library.Name, app.Name, workspaceSize(size.InitialBytes), workspaceSize(size.LazyBytes), workspaceSize(size.TotalBytes))
		}
	}
	if len(libraries) == 0 {
		b.WriteString("No owned library contributions measured.\n")
	}
	if len(libraries) < len(r.Libraries) {
		fmt.Fprintf(&b, "\nShowing %d of %d libraries; use --all for the full list.\n", len(libraries), len(r.Libraries))
	}
	b.WriteString("\n")
	heading("Drill-down")
	if markdown {
		b.WriteString("<details>\n<summary>Commands and artifact paths</summary>\n\n")
	}
	b.WriteString("Run from the Nx workspace root. Commands target each app's largest third-party initial contributor when available.\n\n")
	if markdown {
		b.WriteString("```sh\n")
	}
	fmt.Fprintf(&b, "cd %s\n", workspaceQuote(r.Root))
	for _, app := range r.Apps {
		if app.Analysis == nil {
			continue
		}
		target := ""
		for _, p := range r.Packages {
			if size := p.Apps[app.Name]; size != nil && size.InitialBytes > 0 && !frameworkPackage(p.Name) {
				target = p.Name
				break
			}
		}
		stats, dist := workspaceRelative(r.Root, app.Stats), workspaceRelative(r.Root, app.Dist)
		fmt.Fprintf(&b, "\n# %s\n", strings.NewReplacer("\n", " ", "\r", " ").Replace(app.Name))
		if target != "" {
			fmt.Fprintf(&b, "bundlecheck why --stats %s --dist %s --package %s\n", workspaceQuote(stats), workspaceQuote(dist), workspaceQuote(target))
		}
		fmt.Fprintf(&b, "bundlecheck suggest --stats %s --dist %s\n", workspaceQuote(stats), workspaceQuote(dist))
	}
	if markdown {
		b.WriteString("```\n\n</details>\n")
	}
	if !markdown {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		if _, err := io.WriteString(tw, b.String()); err != nil {
			return err
		}
		return tw.Flush()
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func frameworkPackage(name string) bool {
	return strings.HasPrefix(name, "@angular/") || name == "rxjs" || name == "zone.js" || name == "tslib"
}
func workspaceShare(n int64, app workspace.App) string {
	if app.Analysis == nil || app.Analysis.Summary.InitialJS == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(n)/float64(app.Analysis.Summary.InitialJS))
}
func workspaceSize(n int64) string {
	return strings.NewReplacer(" KB", " KiB", " MB", " MiB", " GB", " GiB", " TB", " TiB", " PB", " PiB", " EB", " EiB").Replace(formatBytes(n))
}
func workspaceRelative(root, p string) string {
	if relative, err := filepath.Rel(root, p); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(relative)
	}
	return p
}
func workspaceQuote(s string) string {
	if s != "" && !strings.ContainsFunc(s, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && !strings.ContainsRune("_./:@=-", r)
	}) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
