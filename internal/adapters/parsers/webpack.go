package parsers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

type WebpackStats struct {
	AssetsByChunkName map[string]any  `json:"assetsByChunkName"`
	Chunks            []WebpackChunk  `json:"chunks"`
}

type WebpackChunk struct {
	ID      any             `json:"id"`
	Names   []string        `json:"names"`
	Files   []string        `json:"files"`
	Size    int64           `json:"size"`
	Initial bool            `json:"initial"`
	Entry   bool            `json:"entry"`
	Modules []WebpackModule `json:"modules"`
}

type WebpackModule struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type WebpackParser struct{}

func (p *WebpackParser) Name() string {
	return "webpack"
}

func (p *WebpackParser) Detect(sample []byte, distDir string) bool {
	return bytes.Contains(sample, []byte(`"assetsByChunkName"`)) ||
		(bytes.Contains(sample, []byte(`"chunks"`)) && bytes.Contains(sample, []byte(`"modules"`)))
}

func (p *WebpackParser) Parse(ctx context.Context, target core.Target) (*core.Bundle, error) {
	data, err := os.ReadFile(target.StatsPath)
	if err != nil {
		return nil, fmt.Errorf("read webpack stats %q: %w", target.StatsPath, err)
	}

	var stats WebpackStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, fmt.Errorf("parse webpack stats JSON: %w", err)
	}

	bundle := core.NewBundle(core.Metadata{
		Bundler: "webpack",
	})

	for _, wChunk := range stats.Chunks {
		chunkName := fmt.Sprintf("%v", wChunk.ID)
		if len(wChunk.Names) > 0 {
			chunkName = wChunk.Names[0]
		}
		fileName := chunkName
		if len(wChunk.Files) > 0 {
			fileName = wChunk.Files[0]
		}

		loadType := core.LoadTypeAsync
		if wChunk.Initial || wChunk.Entry {
			loadType = core.LoadTypeInitial
		}

		chunk := core.Chunk{
			ID:        chunkName,
			Name:      fileName,
			Path:      fileName,
			SizeBytes: wChunk.Size,
			GzipBytes: estimateGzip(wChunk.Size),
			Type:      loadType,
			Entry:     chunkName,
			ModuleIDs: make([]string, 0, len(wChunk.Modules)),
		}

		for _, m := range wChunk.Modules {
			chunk.ModuleIDs = append(chunk.ModuleIDs, m.Name)
			pkgName := extractPackageName(m.Name)

			bundle.AddModule(core.Module{
				ID:        m.Name,
				Package:   pkgName,
				SizeBytes: m.Size,
				GzipBytes: estimateGzip(m.Size),
				IsAppCode: pkgName == "",
				ChunkIDs:  []string{chunkName},
			})
		}

		bundle.AddChunk(chunk)

		if wChunk.Initial || wChunk.Entry {
			bundle.AddEntrypoint(chunkName, core.Entrypoint{
				Name:             chunkName,
				InitialBytes:     wChunk.Size,
				InitialGzipBytes: estimateGzip(wChunk.Size),
				ChunkIDs:         []string{chunkName},
			})
		}
	}

	return bundle, nil
}
