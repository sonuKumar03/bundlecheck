package build

import (
	"bundlecheck/internal/angular"
	"bundlecheck/internal/artifact"
	"bundlecheck/internal/graph"
	"bundlecheck/internal/snapshot"
)

// Load converts build artifacts into the bundler-independent snapshot model.
func Load(stats, dist string) (*snapshot.BundleSnapshot, error) {
	meta, err := angular.Parse(stats)
	if err != nil {
		return nil, err
	}
	s, err := angular.Normalize(meta)
	if err != nil {
		return nil, err
	}
	outputs, roots, err := artifact.BrowserOutputs(s.Outputs, dist)
	if err != nil {
		return nil, err
	}
	if err := graph.Classify(outputs, roots); err != nil {
		return nil, err
	}
	s.Outputs = outputs
	return s, nil
}
