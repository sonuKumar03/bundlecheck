package angular

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"bundlecheck/internal/snapshot"
)

type rawInput struct {
	Bytes   *int64   `json:"bytes"`
	Imports []Import `json:"imports"`
	Format  string   `json:"format,omitempty"`
}

type rawOutputInput struct {
	BytesInOutput *int64 `json:"bytesInOutput"`
}

type rawOutput struct {
	Bytes      *int64                    `json:"bytes"`
	Inputs     map[string]*rawOutputInput `json:"inputs"`
	Imports    []Import                  `json:"imports"`
	Exports    []string                  `json:"exports"`
	EntryPoint string                    `json:"entryPoint,omitempty"`
	CSSBundle  string                    `json:"cssBundle,omitempty"`
}

type rawMetafile struct {
	Inputs  map[string]*rawInput  `json:"inputs"`
	Outputs map[string]*rawOutput `json:"outputs"`
}

func Parse(p string) (*Metafile, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read stats %q: %w", p, err)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	var rm rawMetafile
	if err := dec.Decode(&rm); err != nil {
		return nil, fmt.Errorf("parse stats JSON %q: %w", p, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("parse stats JSON %q: trailing document", p)
	}

	if rm.Inputs == nil {
		return nil, fmt.Errorf("unsupported stats %q: inputs must be a non-null object", p)
	}
	if rm.Outputs == nil {
		return nil, fmt.Errorf("unsupported stats %q: outputs must be a non-null object", p)
	}

	m := &Metafile{
		Inputs:  make(map[string]Input, len(rm.Inputs)),
		Outputs: make(map[string]Output, len(rm.Outputs)),
	}

	known := make(map[string]bool, len(rm.Inputs))

	inputKeys := keys(rm.Inputs)
	for _, k := range inputKeys {
		if k == "" {
			return nil, fmt.Errorf("unsupported stats %q: inputs contains an empty path", p)
		}
		in := rm.Inputs[k]
		if in == nil {
			return nil, fmt.Errorf("unsupported stats %q: inputs[%s] must be a non-null object", p, k)
		}
		if in.Bytes == nil || *in.Bytes < 0 {
			return nil, fmt.Errorf("unsupported stats %q: bytes for %s must be a nonnegative integer", p, k)
		}
		known[snapshot.CleanPath(k)] = true
		m.Inputs[k] = Input{
			Bytes:   *in.Bytes,
			Imports: in.Imports,
			Format:  in.Format,
		}
	}

	outputKeys := keys(rm.Outputs)
	for _, k := range outputKeys {
		if k == "" {
			return nil, fmt.Errorf("unsupported stats %q: outputs contains an empty path", p)
		}
		out := rm.Outputs[k]
		if out == nil {
			return nil, fmt.Errorf("unsupported stats %q: outputs[%s] must be a non-null object", p, k)
		}
		if out.Bytes == nil || *out.Bytes < 0 {
			return nil, fmt.Errorf("unsupported stats %q: bytes for %s must be a nonnegative integer", p, k)
		}
		if out.Inputs == nil {
			return nil, fmt.Errorf("unsupported stats %q: output inputs for %s must be a non-null object", p, k)
		}

		outInputs := make(map[string]OutputInput, len(out.Inputs))
		contributionKeys := keys(out.Inputs)
		for _, inputPath := range contributionKeys {
			if !known[snapshot.CleanPath(inputPath)] {
				return nil, fmt.Errorf("unsupported stats %q: output %q references unknown input %q", p, k, inputPath)
			}
			c := out.Inputs[inputPath]
			if c == nil {
				return nil, fmt.Errorf("unsupported stats %q: contribution for %s must be a non-null object", p, inputPath)
			}
			if c.BytesInOutput == nil || *c.BytesInOutput < 0 {
				return nil, fmt.Errorf("unsupported stats %q: bytesInOutput for %s must be a nonnegative integer", p, inputPath)
			}
			outInputs[inputPath] = OutputInput{
				BytesInOutput: *c.BytesInOutput,
			}
		}

		for _, imp := range out.Imports {
			if imp.Path == "" || imp.Kind == "" {
				return nil, fmt.Errorf("unsupported stats %q: output %q has an import missing path or kind", p, k)
			}
		}

		m.Outputs[k] = Output{
			Bytes:      *out.Bytes,
			Inputs:     outInputs,
			Imports:    out.Imports,
			Exports:    out.Exports,
			EntryPoint: out.EntryPoint,
			CSSBundle:  out.CSSBundle,
		}
	}

	return m, nil
}

func keys[V any](m map[string]V) []string {
	res := make([]string, 0, len(m))
	for k := range m {
		res = append(res, k)
	}
	slices.Sort(res)
	return res
}
