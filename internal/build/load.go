package build

import (
	"github.com/sonuKumar03/bundlecheck/internal/angular"
	"github.com/sonuKumar03/bundlecheck/internal/artifact"
	"github.com/sonuKumar03/bundlecheck/internal/graph"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
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
