package analysis

import (
	"strings"

	"bundlecheck/internal/snapshot"
)

// PackageName selects the innermost node_modules owner, including pnpm layouts.
func PackageName(p string) (string, bool) {
	parts := strings.Split(snapshot.CleanPath(p), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "node_modules" {
			continue
		}
		if i+1 >= len(parts) {
			return "", false
		}
		name := parts[i+1]
		if name == "" || strings.HasPrefix(name, ".") {
			return "", false
		}
		if strings.HasPrefix(name, "@") {
			if len(name) == 1 || i+2 >= len(parts) || parts[i+2] == "" || strings.HasPrefix(parts[i+2], ".") || strings.HasPrefix(parts[i+2], "@") {
				return "", false
			}
			name += "/" + parts[i+2]
		}
		return name, true
	}
	return "", false
}
