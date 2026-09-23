package parsers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

type AngularParser struct{}

func (p *AngularParser) Name() string {
	return "angular"
}

func (p *AngularParser) Detect(sample []byte, distDir string) bool {
	if !bytes.Contains(sample, []byte(`"inputs"`)) {
		return false
	}
	// Check for Angular indicators
	if bytes.Contains(sample, []byte("@angular/")) ||
		bytes.Contains(sample, []byte("zone.js")) ||
		bytes.Contains(sample, []byte("polyfills")) ||
		bytes.Contains(sample, []byte("main.js")) {
		return true
	}
	if distDir != "" {
		if fi, err := os.Stat(filepath.Join(distDir, "index.html")); err == nil && !fi.IsDir() {
			return true
		}
		if fi, err := os.Stat(filepath.Join(distDir, "browser", "index.html")); err == nil && !fi.IsDir() {
			return true
		}
	}
	return false
}

func (p *AngularParser) Parse(ctx context.Context, target core.Target) (*core.Bundle, error) {
	data, err := os.ReadFile(target.StatsPath)
	if err != nil {
		return nil, fmt.Errorf("read angular stats %q: %w", target.StatsPath, err)
	}

	var meta EsbuildMetafile
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse angular stats JSON: %w", err)
	}

	bundle := core.NewBundle(core.Metadata{
		Bundler: "angular",
	})

	var initialChunkIDs []string
	var initialBytes int64
	var initialGzipBytes int64
	var asyncBytes int64

	for outPath, out := range meta.Outputs {
		ext := strings.ToLower(filepath.Ext(outPath))
		if ext != ".js" && ext != ".mjs" && ext != ".css" {
			bundle.AddAsset(core.Asset{
				Path:      outPath,
				SizeBytes: out.Bytes,
				GzipBytes: estimateGzip(out.Bytes),
			})
			continue
		}

		baseName := filepath.Base(outPath)
		isInitial := isAngularInitial(baseName, out.EntryPoint)

		loadType := core.LoadTypeAsync
		if isInitial {
			loadType = core.LoadTypeInitial
			initialChunkIDs = append(initialChunkIDs, baseName)
			initialBytes += out.Bytes
			initialGzipBytes += estimateGzip(out.Bytes)
		} else {
			asyncBytes += out.Bytes
		}

		chunk := core.Chunk{
			ID:        baseName,
			Name:      baseName,
			Path:      outPath,
			SizeBytes: out.Bytes,
			GzipBytes: estimateGzip(out.Bytes),
			Type:      loadType,
			Entry:     out.EntryPoint,
			ModuleIDs: make([]string, 0, len(out.Inputs)),
		}

		for inPath, inBytes := range out.Inputs {
			chunk.ModuleIDs = append(chunk.ModuleIDs, inPath)
			pkgName := extractPackageName(inPath)
			isApp := (pkgName == "")

			bundle.AddModule(core.Module{
				ID:        inPath,
				Package:   pkgName,
				SizeBytes: inBytes.BytesInOutput,
				GzipBytes: estimateGzip(inBytes.BytesInOutput),
				IsAppCode: isApp,
				ChunkIDs:  []string{baseName},
			})
		}

		bundle.AddChunk(chunk)
	}

	entryName := "main"
	bundle.AddEntrypoint(entryName, core.Entrypoint{
		Name:             entryName,
		InitialBytes:     initialBytes,
		InitialGzipBytes: initialGzipBytes,
		AsyncBytes:       asyncBytes,
		ChunkIDs:         initialChunkIDs,
	})

	return bundle, nil
}

func isAngularInitial(baseName, entryPoint string) bool {
	if entryPoint != "" {
		return true
	}
	lower := strings.ToLower(baseName)
	return strings.HasPrefix(lower, "main") ||
		strings.HasPrefix(lower, "polyfills") ||
		strings.HasPrefix(lower, "runtime") ||
		strings.HasPrefix(lower, "styles")
}
