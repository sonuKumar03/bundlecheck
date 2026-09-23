package core

import (
	"sort"
)

// LoadType defines how an asset or chunk is loaded by the browser.
type LoadType string

const (
	LoadTypeInitial LoadType = "initial" // Blocking critical assets for first paint
	LoadTypeAsync   LoadType = "async"   // Dynamically imported lazy chunks
	LoadTypeWorker  LoadType = "worker"  // Web & Service Workers
)

// Metadata captures provenance information about the bundle.
type Metadata struct {
	Bundler   string `json:"bundler"`             // e.g. "esbuild", "vite", "webpack", "angular"
	Version   string `json:"version,omitempty"`   // Bundler version
	CommitSHA string `json:"commitSha,omitempty"` // Git commit SHA if available
	Timestamp int64  `json:"timestamp"`           // Unix timestamp of generation
}

// Bundle is the universal root aggregate representing a compiled web application.
type Bundle struct {
	Metadata    Metadata              `json:"metadata"`
	Entrypoints map[string]Entrypoint `json:"entrypoints"`
	Chunks      []Chunk               `json:"chunks"`
	Modules     []Module              `json:"modules"`
	Assets      []Asset               `json:"assets"`

	chunkIndex  map[string]int
	moduleIndex map[string]int
}

// Entrypoint represents an application entry, route root, or SSR page.
type Entrypoint struct {
	Name             string   `json:"name"`
	InitialBytes     int64    `json:"initialBytes"`
	InitialGzipBytes int64    `json:"initialGzipBytes"`
	AsyncBytes       int64    `json:"asyncBytes"`
	ChunkIDs         []string `json:"chunkIds"`
}

// Chunk represents an emitted JavaScript or stylesheet file.
type Chunk struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	SizeBytes int64    `json:"sizeBytes"`
	GzipBytes int64    `json:"gzipBytes"`
	Type      LoadType `json:"type"`
	Entry     string   `json:"entry,omitempty"`
	ModuleIDs []string `json:"moduleIds"`
}

// Module represents a source file or node_modules package compiled into a chunk.
type Module struct {
	ID           string   `json:"id"`
	Package      string   `json:"package,omitempty"`
	Version      string   `json:"version,omitempty"`
	SizeBytes    int64    `json:"sizeBytes"`
	GzipBytes    int64    `json:"gzipBytes"`
	IsAppCode    bool     `json:"isAppCode"`
	ChunkIDs     []string `json:"chunkIds"`
	IngressPaths []string `json:"ingressPaths,omitempty"`
}

// Asset represents auxiliary compiled files (CSS, WASM, fonts, images).
type Asset struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
	GzipBytes int64  `json:"gzipBytes"`
	MimeType  string `json:"mimeType"`
}

// PackageContribution summarizes a single npm package's impact across a bundle.
type PackageContribution struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
	GzipBytes int64  `json:"gzipBytes"`
	ModuleCount int  `json:"moduleCount"`
}

// NewBundle creates a new Bundle initialized with empty maps.
func NewBundle(meta Metadata) *Bundle {
	return &Bundle{
		Metadata:    meta,
		Entrypoints: make(map[string]Entrypoint),
		Chunks:      make([]Chunk, 0),
		Modules:     make([]Module, 0),
		Assets:      make([]Asset, 0),
		chunkIndex:  make(map[string]int),
		moduleIndex: make(map[string]int),
	}
}

// AddEntrypoint registers an entrypoint into the bundle.
func (b *Bundle) AddEntrypoint(name string, ep Entrypoint) {
	if b.Entrypoints == nil {
		b.Entrypoints = make(map[string]Entrypoint)
	}
	b.Entrypoints[name] = ep
}

// AddChunk appends a chunk to the bundle.
func (b *Bundle) AddChunk(c Chunk) {
	if b.chunkIndex == nil {
		b.chunkIndex = make(map[string]int)
	}
	b.chunkIndex[c.ID] = len(b.Chunks)
	b.Chunks = append(b.Chunks, c)
}

// AddModule appends a module to the bundle.
func (b *Bundle) AddModule(m Module) {
	if b.moduleIndex == nil {
		b.moduleIndex = make(map[string]int)
	}
	b.moduleIndex[m.ID] = len(b.Modules)
	b.Modules = append(b.Modules, m)
}

// AddAsset appends an asset to the bundle.
func (b *Bundle) AddAsset(a Asset) {
	b.Assets = append(b.Assets, a)
}

// TotalInitialBytes returns the sum of initial bytes across all entrypoints.
func (b *Bundle) TotalInitialBytes() int64 {
	var total int64
	for _, ep := range b.Entrypoints {
		total += ep.InitialBytes
	}
	return total
}

// TotalAsyncBytes returns the sum of async bytes across all entrypoints.
func (b *Bundle) TotalAsyncBytes() int64 {
	var total int64
	for _, ep := range b.Entrypoints {
		total += ep.AsyncBytes
	}
	return total
}

// TotalAppCodeBytes returns the sum of sizes of all first-party application modules.
func (b *Bundle) TotalAppCodeBytes() int64 {
	var total int64
	for _, m := range b.Modules {
		if m.IsAppCode {
			total += m.SizeBytes
		}
	}
	return total
}

// FindChunk retrieves a chunk by its ID.
func (b *Bundle) FindChunk(id string) (Chunk, bool) {
	if b.chunkIndex == nil {
		b.chunkIndex = make(map[string]int)
		for i, c := range b.Chunks {
			b.chunkIndex[c.ID] = i
		}
	}
	idx, ok := b.chunkIndex[id]
	if !ok || idx >= len(b.Chunks) {
		return Chunk{}, false
	}
	return b.Chunks[idx], true
}

// FindModule retrieves a module by its ID.
func (b *Bundle) FindModule(id string) (Module, bool) {
	if b.moduleIndex == nil {
		b.moduleIndex = make(map[string]int)
		for i, m := range b.Modules {
			b.moduleIndex[m.ID] = i
		}
	}
	idx, ok := b.moduleIndex[id]
	if !ok || idx >= len(b.Modules) {
		return Module{}, false
	}
	return b.Modules[idx], true
}

// TopPackages calculates the largest npm packages contributing to the bundle.
func (b *Bundle) TopPackages(limit int) []PackageContribution {
	pkgMap := make(map[string]*PackageContribution)
	for _, m := range b.Modules {
		if m.IsAppCode || m.Package == "" {
			continue
		}
		contrib, ok := pkgMap[m.Package]
		if !ok {
			contrib = &PackageContribution{Name: m.Package}
			pkgMap[m.Package] = contrib
		}
		contrib.SizeBytes += m.SizeBytes
		contrib.GzipBytes += m.GzipBytes
		contrib.ModuleCount++
	}

	result := make([]PackageContribution, 0, len(pkgMap))
	for _, c := range pkgMap {
		result = append(result, *c)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].SizeBytes > result[j].SizeBytes
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}
