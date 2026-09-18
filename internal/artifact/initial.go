// Package artifact matches emitted browser files and index script references.
package artifact

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/net/html"

	"bundlecheck/internal/snapshot"
)

func BrowserOutputs(outputs []snapshot.BundleOutput, dist string) ([]snapshot.BundleOutput, []string, error) {
	abs, err := filepath.Abs(dist)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve dist: %w", err)
	}
	index := filepath.Join(abs, "index.html")
	f, err := os.Open(index)
	if err != nil {
		return nil, nil, fmt.Errorf("read index.html %q: %w", index, err)
	}
	defer func() {
		_ = f.Close()
	}()
	doc, err := html.Parse(f)
	if err != nil {
		return nil, nil, fmt.Errorf("parse index.html: %w", err)
	}
	files := make(map[string]bool)
	err = filepath.WalkDir(abs, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !snapshot.IsJavaScript(p) {
			return nil
		}
		rel, err := filepath.Rel(abs, p)
		if err != nil {
			return err
		}
		files[snapshot.CleanPath(rel)] = true
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("inspect browser dist %q: %w", dist, err)
	}
	selected := []snapshot.BundleOutput{}
	byFile := make(map[string]string)
	for _, o := range outputs {
		if !snapshot.IsJavaScript(o.Path) {
			continue
		}
		p := snapshot.CleanPath(o.Path)
		rel, belongs, err := browserPath(p, snapshot.CleanPath(abs), files)
		if err != nil {
			return nil, nil, err
		}
		if !belongs {
			continue
		}
		if !files[rel] {
			return nil, nil, fmt.Errorf("stats output %q has no emitted browser file %q", p, rel)
		}
		if previous, exists := byFile[rel]; exists {
			return nil, nil, fmt.Errorf("ambiguous browser file %q matches stats outputs %q and %q", rel, previous, p)
		}
		byFile[rel] = p
		o.Path = p
		o.DiskPath = filepath.Join(abs, filepath.FromSlash(rel))
		selected = append(selected, o)
	}
	refs, err := scriptPaths(doc)
	if err != nil {
		return nil, nil, err
	}
	roots := []string{}
	seen := make(map[string]bool)
	for _, ref := range refs {
		p, exists := byFile[ref]
		if !exists {
			return nil, nil, fmt.Errorf("local script %q in index.html has no matching emitted browser JS output in stats", ref)
		}
		if !seen[p] {
			roots = append(roots, p)
			seen[p] = true
		}
	}
	if len(roots) == 0 {
		return nil, nil, fmt.Errorf("index.html contains no local browser bootstrap scripts")
	}
	slices.Sort(roots)
	return selected, roots, nil
}

// browserPath accepts dist-relative keys, actual absolute paths, or relocated
// keys whose directory prefix ends in the supplied dist directory name. It
// deliberately avoids basename-only matching of arbitrary prefixes (SSR).
func browserPath(p, dist string, files map[string]bool) (string, bool, error) {
	if strings.HasPrefix(p, dist+"/") {
		return strings.TrimPrefix(p, dist+"/"), true, nil
	}
	candidates := make(map[string]bool)
	if files[p] || !strings.Contains(p, "/") {
		candidates[p] = true
	}
	marker := "/" + path.Base(dist) + "/"
	rest := "/" + p
	for {
		i := strings.Index(rest, marker)
		if i < 0 {
			break
		}
		tail := rest[i+len(marker):]
		candidates[tail] = true
		rest = "/" + tail
	}
	possible := make([]string, 0, len(candidates))
	for candidate := range candidates {
		possible = append(possible, candidate)
	}
	slices.Sort(possible)
	match := ""
	for _, candidate := range possible {
		if !files[candidate] {
			continue
		}
		if match != "" {
			return "", false, fmt.Errorf("ambiguous stats output %q matches browser files %q and %q", p, match, candidate)
		}
		match = candidate
	}
	if match != "" {
		return match, true, nil
	}
	if len(possible) > 0 {
		return possible[0], true, nil
	}
	return "", false, nil
}

func scriptPaths(doc *html.Node) ([]string, error) {
	base := &url.URL{Path: "/"}
	var scripts []*html.Node
	baseFound := false
	var walk func(*html.Node) error
	walk = func(n *html.Node) error {
		if n.Type == html.ElementNode {
			if n.Data == "base" && !baseFound {
				if href, exists := attr(n, "href"); exists {
					u, err := url.Parse(strings.ReplaceAll(href, `\`, "/"))
					if err != nil {
						return fmt.Errorf("invalid index.html base href %q: %w", href, err)
					}
					base = base.ResolveReference(u)
					baseFound = true
				}
			}
			if n.Data == "script" {
				scripts = append(scripts, n)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(doc); err != nil {
		return nil, err
	}
	baseDir := path.Dir(base.Path) + "/"
	if strings.HasSuffix(base.Path, "/") {
		baseDir = base.Path
	}
	refs := []string{}
	for _, n := range scripts {
		typ, _ := attr(n, "type")
		// Script type attributes use a MIME essence match, not MIME parsing:
		// parameters denote data blocks even when the essence is JavaScript.
		switch strings.ToLower(strings.Trim(typ, " \t\n\r\f")) {
		case "", "module", "application/ecmascript", "application/javascript",
			"application/x-ecmascript", "application/x-javascript",
			"text/ecmascript", "text/javascript", "text/javascript1.0",
			"text/javascript1.1", "text/javascript1.2", "text/javascript1.3",
			"text/javascript1.4", "text/javascript1.5", "text/jscript",
			"text/livescript", "text/x-ecmascript", "text/x-javascript":
		default:
			continue
		}
		src, exists := attr(n, "src")
		if !exists || strings.TrimSpace(src) == "" {
			continue
		}
		u, err := url.Parse(strings.ReplaceAll(strings.TrimSpace(src), `\`, "/"))
		if err != nil {
			return nil, fmt.Errorf("invalid index.html script URL %q: %w", src, err)
		}
		u = base.ResolveReference(u)
		if u.Scheme != "" || u.Host != "" {
			continue
		}
		p := snapshot.CleanPath(u.Path)
		p = strings.TrimPrefix(p, baseDir)
		p = strings.TrimPrefix(p, "/")
		refs = append(refs, p)
	}
	return refs, nil
}

func attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}
