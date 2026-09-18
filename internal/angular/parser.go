package angular

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"bundlecheck/internal/snapshot"
)

func Parse(p string) (*Metafile, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read stats %q: %w", p, err)
	}
	var m Metafile
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse stats JSON %q: %w", p, err)
	}
	if err := validate(data, &m); err != nil {
		return nil, fmt.Errorf("unsupported stats %q: %w", p, err)
	}
	return &m, nil
}

func validate(data []byte, m *Metafile) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}
	known := make(map[string]bool)
	for _, field := range []string{"inputs", "outputs"} {
		entries, err := object(root[field], field)
		if err != nil {
			return err
		}
		for _, p := range keys(entries) {
			if p == "" {
				return fmt.Errorf("%s contains an empty path", field)
			}
			entry, err := object(entries[p], field+"["+p+"]")
			if err != nil {
				return err
			}
			if err := byteField(entry["bytes"], "bytes for "+p); err != nil {
				return err
			}
			if field == "inputs" {
				known[snapshot.CleanPath(p)] = true
				continue
			}
			contributions, err := object(entry["inputs"], "output inputs for "+p)
			if err != nil {
				return err
			}
			for _, input := range keys(contributions) {
				if !known[snapshot.CleanPath(input)] {
					return fmt.Errorf("output %q references unknown input %q", p, input)
				}
				c, err := object(contributions[input], "contribution for "+input)
				if err != nil {
					return err
				}
				if err := byteField(c["bytesInOutput"], "bytesInOutput for "+input); err != nil {
					return err
				}
			}
		}
	}
	for _, p := range keys(m.Outputs) {
		for _, imp := range m.Outputs[p].Imports {
			if imp.Path == "" || imp.Kind == "" {
				return fmt.Errorf("output %q has an import missing path or kind", p)
			}
		}
	}
	return nil
}

func object(raw json.RawMessage, name string) (map[string]json.RawMessage, error) {
	var result map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &result) != nil || result == nil {
		return nil, fmt.Errorf("%s must be a non-null object", name)
	}
	return result, nil
}

func byteField(raw json.RawMessage, name string) error {
	var n *int64
	if len(raw) == 0 || json.Unmarshal(raw, &n) != nil || n == nil || *n < 0 {
		return fmt.Errorf("%s must be a nonnegative integer", name)
	}
	return nil
}

func keys[V any](m map[string]V) []string {
	result := make([]string, 0, len(m))
	for key := range m {
		result = append(result, key)
	}
	slices.Sort(result)
	return result
}
