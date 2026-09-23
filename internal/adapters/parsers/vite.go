package parsers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

type ViteChunk struct {
	File           string   `json:"file"`
	Src            string   `json:"src"`
	IsEntry        bool     `json:"isEntry"`
	IsDynamicEntry bool     `json:"isDynamicEntry"`
	Imports        []string `json:"imports"`
	DynamicImports []string `json:"dynamicImports"`
	Css            []string `json:"css"`
	Assets         []string `json:"assets"`
}

type ViteParser struct{}

func (p *ViteParser) Name() string {
	return "vite"
}

func (p *ViteParser) Detect(sample []byte, distDir string) bool {
	// Vite manifests have "file", "isEntry", and typically "src"
	return bytes.Contains(sample, []byte(`"file"`)) &&
		(bytes.Contains(sample, []byte(`"isEntry"`)) || bytes.Contains(sample, []byte(`"isDynamicEntry"`)))
}

func (p *ViteParser) Parse(ctx context.Context, target core.Target) (*core.Bundle, error) {
	data, err := os.ReadFile(target.StatsPath)
	if err != nil {
		return nil, fmt.Errorf("read vite manifest %q: %w", target.StatsPath, err)
	}

	var manifest map[string]ViteChunk
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse vite manifest JSON: %w", err)
	}

	bundle := core.NewBundle(core.Metadata{
		Bundler: "vite",
	})

	distDir := target.DistPath
	if distDir == "" {
		distDir = filepath.Dir(target.StatsPath)
	}

	for srcKey, vChunk := range manifest {
		filePath := filepath.Join(distDir, vChunk.File)
		var sizeBytes int64
		if fi, err := os.Stat(filePath); err == nil {
			sizeBytes = fi.Size()
		}

		loadType := core.LoadTypeAsync
		if vChunk.IsEntry {
			loadType = core.LoadTypeInitial
		}

		chunkID := filepath.Base(vChunk.File)
		chunk := core.Chunk{
			ID:        chunkID,
			Name:      chunkID,
			Path:      vChunk.File,
			SizeBytes: sizeBytes,
			GzipBytes: estimateGzip(sizeBytes),
			Type:      loadType,
			Entry:     srcKey,
			ModuleIDs: []string{srcKey},
		}

		pkgName := extractPackageName(srcKey)
		bundle.AddModule(core.Module{
			ID:        srcKey,
			Package:   pkgName,
			SizeBytes: sizeBytes,
			GzipBytes: estimateGzip(sizeBytes),
			IsAppCode: pkgName == "",
			ChunkIDs:  []string{chunkID},
		})

		bundle.AddChunk(chunk)

		if vChunk.IsEntry {
			bundle.AddEntrypoint(srcKey, core.Entrypoint{
				Name:             srcKey,
				InitialBytes:     sizeBytes,
				InitialGzipBytes: estimateGzip(sizeBytes),
				ChunkIDs:         []string{chunkID},
			})
		}

		// Add emitted CSS as assets
		for _, cssFile := range vChunk.Css {
			cssPath := filepath.Join(distDir, cssFile)
			var cssSize int64
			if fi, err := os.Stat(cssPath); err == nil {
				cssSize = fi.Size()
			}
			bundle.AddAsset(core.Asset{
				Path:      cssFile,
				SizeBytes: cssSize,
				GzipBytes: estimateGzip(cssSize),
				MimeType:  "text/css",
			})
		}
	}

	return bundle, nil
}
