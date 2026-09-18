// Package angular adapts Angular's esbuild metadata to a portable snapshot.
package angular

type Metafile struct {
	Inputs  map[string]Input  `json:"inputs"`
	Outputs map[string]Output `json:"outputs"`
}

type Input struct {
	Bytes   int64    `json:"bytes"`
	Imports []Import `json:"imports"`
	Format  string   `json:"format,omitempty"`
}

type Import struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	External bool   `json:"external,omitempty"`
}

type Output struct {
	Bytes      int64                  `json:"bytes"`
	Inputs     map[string]OutputInput `json:"inputs"`
	Imports    []Import               `json:"imports"`
	Exports    []string               `json:"exports"`
	EntryPoint string                 `json:"entryPoint,omitempty"`
	CSSBundle  string                 `json:"cssBundle,omitempty"`
}

type OutputInput struct {
	BytesInOutput int64 `json:"bytesInOutput"`
}
