// Package snapshot defines the framework-independent bundle model.
package snapshot

import (
	"path"
	"strings"
)

type BundleSnapshot struct {
	SchemaVersion string         `json:"schemaVersion"`
	Totals        Totals         `json:"totals"`
	Inputs        []Module       `json:"inputs"`
	Outputs       []BundleOutput `json:"outputs"`
	Packages      []Package      `json:"packages"`
}

type Totals struct {
	InitialJS     int64 `json:"initialJs"`
	InitialGzipJS int64 `json:"initialGzipJs,omitempty"`
	LazyJS        int64 `json:"lazyJs"`
	LazyGzipJS    int64 `json:"lazyGzipJs,omitempty"`
	TotalJS       int64 `json:"totalJs"`
	TotalGzipJS   int64 `json:"totalGzipJs,omitempty"`
}

type Module struct {
	Path    string   `json:"path"`
	Bytes   int64    `json:"bytes"`
	Imports []Import `json:"imports,omitempty"`
}

type BundleOutput struct {
	Path       string         `json:"path"`
	Bytes      int64          `json:"bytes"`
	GzipBytes  int64          `json:"gzipBytes,omitempty"`
	Initial    bool           `json:"initial"`
	EntryPoint string         `json:"entryPoint,omitempty"`
	Inputs     []Contribution `json:"inputs"`
	Imports    []Import       `json:"imports"`
}

type Contribution struct {
	Input string `json:"input"`
	Bytes int64  `json:"bytes"`
}

type Import struct {
	Path     string `json:"path"`
	Dynamic  bool   `json:"dynamic"`
	External bool   `json:"external"`
	// Asset references identify emitted files without importing their JS code.
	Asset bool `json:"asset,omitempty"`
}

type Package struct {
	Name             string `json:"name"`
	InitialBytes     int64  `json:"initialBytes"`
	InitialGzipBytes int64  `json:"initialGzipBytes,omitempty"`
	LazyBytes        int64  `json:"lazyBytes"`
	LazyGzipBytes    int64  `json:"lazyGzipBytes,omitempty"`
	TotalBytes       int64  `json:"totalBytes"`
	TotalGzipBytes   int64  `json:"totalGzipBytes,omitempty"`
}

// CleanPath uses portable slash-separated identifiers, even for Windows stats.
func CleanPath(p string) string { return path.Clean(strings.ReplaceAll(p, `\`, "/")) }

func IsJavaScript(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".js", ".mjs", ".cjs":
		return true
	}
	return false
}
