package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

// BundleDTO represents the normalized bundle AST payload returned by the studio API.
type BundleDTO struct {
	Bundler     string                   `json:"bundler,omitempty"`
	StatsPath   string                   `json:"statsPath"`
	Entrypoints map[string]EntrypointDTO `json:"entrypoints"`
	Chunks      []ChunkDTO               `json:"chunks"`
	TopPackages []PackageDTO             `json:"topPackages"`
	TotalBytes  int64                    `json:"totalBytes"`
}

// EntrypointDTO represents entrypoint sizes and constituent chunks.
type EntrypointDTO struct {
	InitialBytes     int64    `json:"initialBytes"`
	InitialGzipBytes int64    `json:"initialGzipBytes"`
	AsyncBytes       int64    `json:"asyncBytes"`
	ChunkIds         []string `json:"chunkIds"`
}

// ChunkDTO represents an emitted chunk artifact.
type ChunkDTO struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // "initial" or "async"
	SizeBytes int64  `json:"sizeBytes"`
	GzipBytes int64  `json:"gzipBytes"`
}

// PackageDTO aggregates module metrics and BFS ingress attribution for a single package.
type PackageDTO struct {
	Name        string   `json:"name"`
	SizeBytes   int64    `json:"sizeBytes"`
	GzipBytes   int64    `json:"gzipBytes"`
	Chunks      []string `json:"chunks"`
	IngressPath string   `json:"ingressPath"`
}

func (s *Server) handleGetBundle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	bundle := s.Bundle()
	statsPath := s.statsPath

	if bundle == nil && statsPath != "" {
		scanned, err := s.client.Scan(r.Context(), bundleradar.ScanOptions{
			StatsPath: statsPath,
		})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("failed to parse bundle stats: %v", err),
			})
			return
		}
		s.SetBundle(scanned)
		bundle = scanned
	}

	if bundle == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "No stats.json has been scanned yet",
		})
		return
	}

	var totalBytes int64
	chunks := make([]ChunkDTO, 0, len(bundle.Chunks))
	for _, ch := range bundle.Chunks {
		totalBytes += ch.SizeBytes
		chunks = append(chunks, ChunkDTO{
			Name:      ch.Name,
			Type:      string(ch.Type),
			SizeBytes: ch.SizeBytes,
			GzipBytes: ch.GzipBytes,
		})
	}
	if totalBytes == 0 {
		totalBytes = bundle.TotalInitialBytes() + bundle.TotalAsyncBytes()
	}

	entrypoints := make(map[string]EntrypointDTO, len(bundle.Entrypoints))
	for name, ep := range bundle.Entrypoints {
		entrypoints[name] = EntrypointDTO{
			InitialBytes:     ep.InitialBytes,
			InitialGzipBytes: ep.InitialGzipBytes,
			AsyncBytes:       ep.AsyncBytes,
			ChunkIds:         ep.ChunkIDs,
		}
	}

	// Aggregate and sort packages
	pkgMap := make(map[string]*PackageDTO)
	bestPaths := make(map[string][]string)

	for _, mod := range bundle.Modules {
		pkgName := mod.Package
		if pkgName == "" {
			pkgName = "(application code)"
		}
		p, exists := pkgMap[pkgName]
		if !exists {
			p = &PackageDTO{
				Name:   pkgName,
				Chunks: make([]string, 0),
			}
			pkgMap[pkgName] = p
		}
		p.SizeBytes += mod.SizeBytes
		p.GzipBytes += mod.GzipBytes
		for _, cid := range mod.ChunkIDs {
			if !contains(p.Chunks, cid) {
				p.Chunks = append(p.Chunks, cid)
			}
		}

		if pkgName != "(application code)" && len(mod.IngressPaths) > 0 {
			bp := bestPaths[pkgName]
			if len(bp) == 0 || len(mod.IngressPaths) < len(bp) {
				bestPaths[pkgName] = mod.IngressPaths
			}
		}
	}

	topPackages := make([]PackageDTO, 0, len(pkgMap))
	for pkgName, p := range pkgMap {
		sort.Strings(p.Chunks)
		if bp, ok := bestPaths[pkgName]; ok && len(bp) > 0 {
			p.IngressPath = strings.Join(bp, " → ")
		}
		topPackages = append(topPackages, *p)
	}

	sort.Slice(topPackages, func(i, j int) bool {
		if topPackages[i].SizeBytes != topPackages[j].SizeBytes {
			return topPackages[i].SizeBytes > topPackages[j].SizeBytes
		}
		return topPackages[i].Name < topPackages[j].Name
	})

	dto := BundleDTO{
		Bundler:     bundle.Metadata.Bundler,
		StatsPath:   statsPath,
		Entrypoints: entrypoints,
		Chunks:      chunks,
		TopPackages: topPackages,
		TotalBytes:  totalBytes,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dto)
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
