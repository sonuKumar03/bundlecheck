package core

import (
	"context"
	"io"
)

// Target represents an application target to be parsed or checked.
type Target struct {
	Name      string `json:"name"`
	StatsPath string `json:"statsPath"`
	DistPath  string `json:"distPath,omitempty"`
	Bundler   string `json:"bundler,omitempty"` // Optional override: "esbuild", "angular", "vite", "webpack"
}

// Parser defines the pluggable contract for parsing bundler outputs into a Bundle AST.
type Parser interface {
	Name() string
	Detect(statsContent []byte, distDir string) bool
	Parse(ctx context.Context, target Target) (*Bundle, error)
}

// WorkspaceResolver discovers application targets across a repository or directory.
type WorkspaceResolver interface {
	Name() string
	Detect(root string) bool
	Resolve(ctx context.Context, root string) ([]Target, error)
}

// BaselineProvider retrieves and stores historical bundle snapshots.
type BaselineProvider interface {
	Name() string
	Fetch(ctx context.Context, ref string) (*Bundle, error)
	Save(ctx context.Context, bundle *Bundle, dest string) error
}

// Reporter renders analysis, comparison, or gate results into a designated format.
type Reporter interface {
	Format() string // "terminal", "markdown", "github-pr", "json", "html"
	Render(ctx context.Context, w io.Writer, data any) error
}
