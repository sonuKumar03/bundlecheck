package diff

import (
	"math"
	"sort"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

// Options specifies parameters for bundle diffing.
type Options struct {
	DriftThreshold int64 // Threshold in bytes under which module delta is classified as micro-drift
}

// BundleDiff contains deltas between base and current bundles.
type BundleDiff struct {
	Entrypoints     map[string]EntrypointDelta `json:"entrypoints"`
	Packages        []PackageDelta             `json:"packages"`
	AddedChunks     []core.Chunk               `json:"addedChunks"`
	RemovedChunks   []core.Chunk               `json:"removedChunks"`
	MicroDriftBytes int64                      `json:"microDriftBytes"`
	Attributions    []Attribution              `json:"attributions"`
}

// EntrypointDelta captures size changes for a specific entrypoint.
type EntrypointDelta struct {
	Name             string `json:"name"`
	InitialDelta     int64  `json:"initialDeltaBytes"`
	InitialGzipDelta int64  `json:"initialGzipDeltaBytes"`
	AsyncDelta       int64  `json:"asyncDeltaBytes"`
}

// PackageDelta captures size changes for an npm package.
type PackageDelta struct {
	Name       string `json:"name"`
	DeltaBytes int64  `json:"deltaBytes"`
	GzipDelta  int64  `json:"gzipDeltaBytes"`
	BaseBytes  int64  `json:"baseBytes"`
	CurrBytes  int64  `json:"currBytes"`
}

// Attribution attributes a regression to a source module and reason.
type Attribution struct {
	SourceFile string `json:"sourceFile"`
	Reason     string `json:"reason"`
	DeltaBytes int64  `json:"deltaBytes"`
}

// Calculate computes comprehensive differences between a base bundle and a current bundle.
func Calculate(base, current *core.Bundle, opts Options) *BundleDiff {
	res := &BundleDiff{
		Entrypoints:   make(map[string]EntrypointDelta),
		Packages:      make([]PackageDelta, 0),
		AddedChunks:   make([]core.Chunk, 0),
		RemovedChunks: make([]core.Chunk, 0),
		Attributions:  make([]Attribution, 0),
	}

	if base == nil && current == nil {
		return res
	}

	if base == nil {
		base = core.NewBundle(core.Metadata{})
	}
	if current == nil {
		current = core.NewBundle(core.Metadata{})
	}

	// 1. Entrypoint deltas
	allEntryNames := make(map[string]bool)
	for name := range base.Entrypoints {
		allEntryNames[name] = true
	}
	for name := range current.Entrypoints {
		allEntryNames[name] = true
	}

	for name := range allEntryNames {
		baseEP := base.Entrypoints[name]
		currEP := current.Entrypoints[name]

		res.Entrypoints[name] = EntrypointDelta{
			Name:             name,
			InitialDelta:     currEP.InitialBytes - baseEP.InitialBytes,
			InitialGzipDelta: currEP.InitialGzipBytes - baseEP.InitialGzipBytes,
			AsyncDelta:       currEP.AsyncBytes - baseEP.AsyncBytes,
		}
	}

	// 2. Chunks added / removed
	baseChunkMap := make(map[string]core.Chunk)
	for _, c := range base.Chunks {
		baseChunkMap[c.ID] = c
	}
	currChunkMap := make(map[string]core.Chunk)
	for _, c := range current.Chunks {
		currChunkMap[c.ID] = c
		if _, exists := baseChunkMap[c.ID]; !exists {
			res.AddedChunks = append(res.AddedChunks, c)
		}
	}
	for _, c := range base.Chunks {
		if _, exists := currChunkMap[c.ID]; !exists {
			res.RemovedChunks = append(res.RemovedChunks, c)
		}
	}

	// 3. Package deltas
	basePkgMap := make(map[string]core.PackageContribution)
	for _, p := range base.TopPackages(0) {
		basePkgMap[p.Name] = p
	}
	currPkgMap := make(map[string]core.PackageContribution)
	for _, p := range current.TopPackages(0) {
		currPkgMap[p.Name] = p
	}

	allPkgNames := make(map[string]bool)
	for name := range basePkgMap {
		allPkgNames[name] = true
	}
	for name := range currPkgMap {
		allPkgNames[name] = true
	}

	for name := range allPkgNames {
		bSize := basePkgMap[name].SizeBytes
		bGzip := basePkgMap[name].GzipBytes
		cSize := currPkgMap[name].SizeBytes
		cGzip := currPkgMap[name].GzipBytes

		delta := cSize - bSize
		if delta != 0 {
			res.Packages = append(res.Packages, PackageDelta{
				Name:       name,
				DeltaBytes: delta,
				GzipDelta:  cGzip - bGzip,
				BaseBytes:  bSize,
				CurrBytes:  cSize,
			})
		}
	}

	sort.Slice(res.Packages, func(i, j int) bool {
		return math.Abs(float64(res.Packages[i].DeltaBytes)) > math.Abs(float64(res.Packages[j].DeltaBytes))
	})

	// 4. Module diff & Micro-drift
	baseModMap := make(map[string]core.Module)
	for _, m := range base.Modules {
		baseModMap[m.ID] = m
	}
	currModMap := make(map[string]core.Module)
	for _, m := range current.Modules {
		currModMap[m.ID] = m
	}

	allModIDs := make(map[string]bool)
	for id := range baseModMap {
		allModIDs[id] = true
	}
	for id := range currModMap {
		allModIDs[id] = true
	}

	for id := range allModIDs {
		delta := currModMap[id].SizeBytes - baseModMap[id].SizeBytes
		if delta == 0 {
			continue
		}
		if opts.DriftThreshold > 0 && delta > 0 && delta < opts.DriftThreshold {
			res.MicroDriftBytes += delta
		}
	}

	return res
}
