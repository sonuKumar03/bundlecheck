// Package compression computes and estimates compressed wire transfer sizes (Gzip).
package compression

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/snapshot"
)

const DefaultFallbackRatio = 0.32

// MeasureBytes returns the exact gzip compressed size of the given byte slice.
func MeasureBytes(data []byte) int64 {
	if len(data) == 0 {
		return 0
	}
	var buf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
	if err != nil {
		return int64(float64(len(data)) * DefaultFallbackRatio)
	}
	_, _ = gw.Write(data)
	_ = gw.Close()
	return int64(buf.Len())
}

// MeasureFile returns the gzip compressed size of a file on disk.
func MeasureFile(filePath string) (int64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var buf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
	if err != nil {
		return 0, err
	}
	if _, err := io.Copy(gw, f); err != nil {
		_ = gw.Close()
		return 0, err
	}
	if err := gw.Close(); err != nil {
		return 0, err
	}
	return int64(buf.Len()), nil
}

// AttachCompression measures or estimates gzip sizes for outputs, totals, and packages.
func AttachCompression(s *snapshot.BundleSnapshot, distDir string) {
	if s == nil {
		return
	}

	var (
		initialGzip int64
		lazyGzip    int64
		totalGzip   int64
	)

	// Map each output to its ratio
	outputRatios := make(map[string]float64)

	for i := range s.Outputs {
		o := &s.Outputs[i]
		if !snapshot.IsJavaScript(o.Path) {
			continue
		}

		var gzSize int64
		diskPath := o.DiskPath
		if diskPath == "" {
			diskPath = filepath.Join(distDir, o.Path)
		}
		if fi, err := os.Stat(diskPath); err == nil && !fi.IsDir() && fi.Size() > 0 {
			if measured, err := MeasureFile(diskPath); err == nil && measured > 0 {
				gzSize = measured
			}
		}

		if gzSize == 0 && o.Bytes > 0 {
			// Estimate based on fallback ratio
			gzSize = int64(float64(o.Bytes) * DefaultFallbackRatio)
		}
		if gzSize == 0 && o.Bytes == 0 {
			gzSize = 0
		}

		o.GzipBytes = gzSize
		ratio := DefaultFallbackRatio
		if o.Bytes > 0 {
			ratio = float64(gzSize) / float64(o.Bytes)
		}
		outputRatios[snapshot.CleanPath(o.Path)] = ratio

		if o.Initial {
			initialGzip += gzSize
		} else {
			lazyGzip += gzSize
		}
		totalGzip += gzSize
	}

	s.Totals.InitialGzipJS = initialGzip
	s.Totals.LazyGzipJS = lazyGzip
	s.Totals.TotalGzipJS = totalGzip

	// Update package gzip estimates based on output contributions in a single pass O(C)
	type pkgGzipAcc struct {
		initial int64
		lazy    int64
	}
	pkgAcc := make(map[string]*pkgGzipAcc, len(s.Packages))

	for _, o := range s.Outputs {
		ratio, hasRatio := outputRatios[snapshot.CleanPath(o.Path)]
		if !hasRatio {
			ratio = DefaultFallbackRatio
		}

		for _, c := range o.Inputs {
			inputPath := snapshot.CleanPath(c.Input)
			if pName, isPkg := analysis.PackageName(inputPath); isPkg {
				pNameKey := strings.ToLower(pName)
				acc := pkgAcc[pNameKey]
				if acc == nil {
					acc = &pkgGzipAcc{}
					pkgAcc[pNameKey] = acc
				}
				estGzip := int64(float64(c.Bytes) * ratio)
				if o.Initial {
					acc.initial += estGzip
				} else {
					acc.lazy += estGzip
				}
			}
		}
	}

	for i := range s.Packages {
		pkg := &s.Packages[i]
		var (
			pkgInitialGzip int64
			pkgLazyGzip    int64
		)

		if acc, ok := pkgAcc[strings.ToLower(pkg.Name)]; ok {
			pkgInitialGzip = acc.initial
			pkgLazyGzip = acc.lazy
		}

		if pkgInitialGzip == 0 && pkg.InitialBytes > 0 {
			pkgInitialGzip = int64(float64(pkg.InitialBytes) * DefaultFallbackRatio)
		}
		if pkgLazyGzip == 0 && pkg.LazyBytes > 0 {
			pkgLazyGzip = int64(float64(pkg.LazyBytes) * DefaultFallbackRatio)
		}

		pkg.InitialGzipBytes = pkgInitialGzip
		pkg.LazyGzipBytes = pkgLazyGzip
		pkg.TotalGzipBytes = pkgInitialGzip + pkgLazyGzip
	}
}
