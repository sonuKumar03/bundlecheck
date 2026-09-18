package report

import (
	"encoding/json"
	"io"
)

func JSON(w io.Writer, r any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(r)
}
