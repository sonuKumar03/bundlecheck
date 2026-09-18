// Package graph classifies outputs using normalized dependency edges.
package graph

import (
	"fmt"
	"path"

	"bundlecheck/internal/snapshot"
)

// Classify marks the static closure of roots as initial. Dynamic imports are
// resolved for consistency but never followed by the initial traversal.
func Classify(outputs []snapshot.BundleOutput, roots []string) error {
	if len(roots) == 0 {
		return fmt.Errorf("no browser bootstrap roots")
	}
	byPath := make(map[string]int, len(outputs))
	for i, o := range outputs {
		p := snapshot.CleanPath(o.Path)
		if _, exists := byPath[p]; exists {
			return fmt.Errorf("duplicate output %q", p)
		}
		byPath[p] = i
		outputs[i].Initial = false
	}
	edges := make(map[string][]string)
	for _, o := range outputs {
		from := snapshot.CleanPath(o.Path)
		for _, imp := range o.Imports {
			if imp.External || !snapshot.IsJavaScript(imp.Path) {
				continue
			}
			to := snapshot.CleanPath(imp.Path)
			if _, exists := byPath[to]; !exists {
				to = snapshot.CleanPath(path.Join(path.Dir(from), to))
				if _, exists := byPath[to]; !exists {
					return fmt.Errorf("output %q imports missing browser JS output %q", from, imp.Path)
				}
			}
			if !imp.Dynamic && !imp.Asset {
				edges[from] = append(edges[from], to)
			}
		}
	}
	stack := append([]string(nil), roots...)
	visited := make(map[string]bool)
	for len(stack) > 0 {
		p := snapshot.CleanPath(stack[len(stack)-1])
		stack = stack[:len(stack)-1]
		if visited[p] {
			continue
		}
		i, exists := byPath[p]
		if !exists {
			return fmt.Errorf("bootstrap script %q has no browser JS output", p)
		}
		visited[p] = true
		outputs[i].Initial = true
		stack = append(stack, edges[p]...)
	}
	return nil
}
