package build

import (
	"github.com/sonuKumar03/bundleradar/internal/angular"
	"github.com/sonuKumar03/bundleradar/internal/artifact"
	"github.com/sonuKumar03/bundleradar/internal/graph"
	"github.com/sonuKumar03/bundleradar/internal/snapshot"
)

// LoadWithEntry converts build artifacts into the bundler-independent snapshot model using the specified entry.
func LoadWithEntry(stats, dist, entry string) (*snapshot.BundleSnapshot, error) {
	meta, err := angular.Parse(stats)
	if err != nil {
		return nil, err
	}
	s, err := angular.Normalize(meta)
	if err != nil {
		return nil, err
	}
	outputs, roots, err := artifact.BrowserOutputsWithEntry(s.Outputs, dist, entry)
	if err != nil {
		return nil, err
	}
	if err := graph.Classify(outputs, roots); err != nil {
		return nil, err
	}
	s.Outputs = outputs
	return s, nil
}

// Load converts build artifacts into the bundler-independent snapshot model.
func Load(stats, dist string) (*snapshot.BundleSnapshot, error) {
	return LoadWithEntry(stats, dist, "")
}
