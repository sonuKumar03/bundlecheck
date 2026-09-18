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
	InitialJS int64 `json:"initialJs"`
	LazyJS    int64 `json:"lazyJs"`
	TotalJS   int64 `json:"totalJs"`
}

type Module struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

type BundleOutput struct {
	Path       string         `json:"path"`
	Bytes      int64          `json:"bytes"`
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
	Name         string `json:"name"`
	InitialBytes int64  `json:"initialBytes"`
	LazyBytes    int64  `json:"lazyBytes"`
	TotalBytes   int64  `json:"totalBytes"`
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
