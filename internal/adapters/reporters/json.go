package reporters

import (
	"context"
	"encoding/json"
	"io"
)

type JSONReporter struct{}

func (r *JSONReporter) Format() string {
	return "json"
}

func (r *JSONReporter) Render(ctx context.Context, w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
