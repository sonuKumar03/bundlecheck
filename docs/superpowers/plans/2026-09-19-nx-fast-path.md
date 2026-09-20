# Fast-Path Static Nx Metadata Parser Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a sub-10ms pure-Go static reader for modern Nx workspaces that discovers and parses `project.json` files directly with transparent fallback to `node nx graph --print`.

**Architecture:** A bounded directory walker (depth <= 4) scans for `project.json` while skipping `.git`, `node_modules`, `dist`, `.nx`, etc. It parses each project's targets into `workspace.Project` models, validates that at least one supported Angular application exists, and falls back to Nx CLI if unsupported or dynamic plugins are encountered.

**Tech Stack:** Go stdlib (`os`, `path/filepath`, `encoding/json`, `io/fs`).

**Spec:** `docs/superpowers/specs/2026-09-19-nx-fast-path-design.md`

## Global Constraints
- Pure Go standard library for static parsing (zero external dependencies).
- Bounded traversal depth <= 4 to prevent deep scans or recursive symlink loops.
- 100% backward-compatible: exact match with `workspace.Metadata` schema.
- Automatic transparent fallback to `readMetadataNxCli` if zero supported applications are detected or on parse errors.
- Always prefix all shell/terminal commands with `rtk ` when executed via `run_command`.

---

### Task 1: Implement `internal/workspace/static.go`

**Files:**
- Create: `internal/workspace/static.go`
- Test: `internal/workspace/static_test.go`

**Interfaces:**
- Consumes: `workspace.Metadata`, `workspace.Project`, `workspace.Target`, `workspace.IsApplication`, `workspace.Supported`.
- Produces: `ReadMetadataStatic(root string) (Metadata, error)`, `ErrStaticFallbackRequired`.

- [x] **Step 1: Write the failing unit test**

Create `internal/workspace/static_test.go`:
```go
package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadMetadataStatic(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	testWorkspace := filepath.Join(repoRoot, "testdata", "nx-workspace")

	if _, err := os.Stat(filepath.Join(testWorkspace, "nx.json")); os.IsNotExist(err) {
		t.Skip("testdata/nx-workspace not found")
	}

	m, err := ReadMetadataStatic(testWorkspace)
	if err != nil {
		t.Fatalf("ReadMetadataStatic failed: %v", err)
	}

	if len(m.Graph.Nodes) == 0 {
		t.Fatalf("expected discovered nodes, got 0")
	}

	admin, ok := m.Graph.Nodes["admin-dashboard"]
	if !ok {
		t.Fatalf("expected admin-dashboard project in static metadata")
	}

	if !IsApplication(admin) {
		t.Errorf("expected admin-dashboard to be an application")
	}

	if admin.Data.Root != filepath.FromSlash("apps/admin-dashboard") {
		t.Errorf("expected root apps/admin-dashboard, got %q", admin.Data.Root)
	}

	portal, ok := m.Graph.Nodes["portal"]
	if !ok {
		t.Fatalf("expected portal project in static metadata")
	}

	if !IsApplication(portal) {
		t.Errorf("expected portal to be an application")
	}

	charting, ok := m.Graph.Nodes["charting"]
	if !ok {
		t.Fatalf("expected charting project in static metadata")
	}
	if IsApplication(charting) {
		t.Errorf("expected charting to be a library, not application")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/workspace -run TestReadMetadataStatic`
Expected: FAIL with undefined `ReadMetadataStatic`

- [x] **Step 3: Implement `internal/workspace/static.go`**

Write `internal/workspace/static.go`:
```go
package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrStaticFallbackRequired is returned when static parsing cannot satisfy the workspace model.
var ErrStaticFallbackRequired = errors.New("static parsing cannot satisfy workspace model; Nx CLI fallback required")

type rawProjectFile struct {
	Name        string            `json:"name"`
	ProjectType string            `json:"projectType"`
	Targets     map[string]Target `json:"targets"`
}

// ReadMetadataStatic scans the workspace root for modern project.json manifests without running Node.js.
func ReadMetadataStatic(root string) (Metadata, error) {
	var m Metadata
	m.Graph.Nodes = make(map[string]Project)

	if fi, err := os.Stat(filepath.Join(root, "nx.json")); err != nil || fi.IsDir() {
		return m, fmt.Errorf("nx.json not found in %s: %w", root, err)
	}

	cleanRoot := filepath.Clean(root)
	maxDepth := 4

	err := filepath.WalkDir(cleanRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if path != cleanRoot && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "dist" || name == "coverage" || name == "tmp") {
				return filepath.SkipDir
			}

			rel, relErr := filepath.Rel(cleanRoot, path)
			if relErr == nil && rel != "." {
				depth := len(strings.Split(rel, string(filepath.Separator)))
				if depth > maxDepth {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if d.Name() == "project.json" {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			var pf rawProjectFile
			if err := json.Unmarshal(data, &pf); err != nil {
				return nil
			}

			dir := filepath.Dir(path)
			relRoot, err := filepath.Rel(cleanRoot, dir)
			if err != nil {
				return nil
			}

			projName := pf.Name
			if projName == "" {
				projName = filepath.Base(dir)
			}

			p := Project{
				Type: "lib",
			}
			pType := strings.ToLower(pf.ProjectType)
			if pType == "application" || pType == "app" {
				p.Type = "app"
			}
			p.Data.Root = filepath.ToSlash(relRoot)
			p.Data.ProjectType = pf.ProjectType
			if p.Data.ProjectType == "" {
				if p.Type == "app" {
					p.Data.ProjectType = "application"
				} else {
					p.Data.ProjectType = "library"
				}
			}
			p.Data.Targets = pf.Targets

			m.Graph.Nodes[projName] = p
		}
		return nil
	})

	if err != nil {
		return m, err
	}

	if len(m.Graph.Nodes) == 0 {
		return m, ErrStaticFallbackRequired
	}

	hasApp := false
	for _, p := range m.Graph.Nodes {
		if IsApplication(p) {
			hasApp = true
			break
		}
	}

	if !hasApp {
		return m, ErrStaticFallbackRequired
	}

	return m, nil
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `rtk go test -v ./internal/workspace -run TestReadMetadataStatic`
Expected: PASS

- [x] **Step 5: Commit Task 1**

```bash
rtk git add internal/workspace/static.go internal/workspace/static_test.go
rtk git commit -m "feat(workspace): implement pure-Go static metadata parser for project.json"
```

---

### Task 2: Connect Fast-Path Reader to `ReadMetadata` with Transparent Fallback

**Files:**
- Modify: `internal/workspace/nx.go:53-97`
- Test: `internal/workspace/nx_test.go`

**Interfaces:**
- Consumes: `ReadMetadataStatic`, `readMetadataNxCli`.
- Produces: `ReadMetadata(ctx context.Context, root string) (Metadata, error)`.

- [x] **Step 1: Write test verifying transparent fallback and equivalence**

Add to `internal/workspace/static_test.go`:
```go
func TestReadMetadataEquivalence(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	testWorkspace := filepath.Join(repoRoot, "testdata", "nx-workspace")

	if _, err := os.Stat(filepath.Join(testWorkspace, "node_modules", "nx")); os.IsNotExist(err) {
		t.Skip("testdata/nx-workspace/node_modules/nx not installed")
	}

	cliMeta, err := readMetadataNxCli(context.Background(), testWorkspace)
	if err != nil {
		t.Skipf("Nx CLI unavailable: %v", err)
	}

	staticMeta, err := ReadMetadataStatic(testWorkspace)
	if err != nil {
		t.Fatalf("ReadMetadataStatic failed: %v", err)
	}

	for name, cliProj := range cliMeta.Graph.Nodes {
		if !IsApplication(cliProj) {
			continue
		}
		statProj, ok := staticMeta.Graph.Nodes[name]
		if !ok {
			t.Errorf("missing app %q in static metadata", name)
			continue
		}
		if statProj.Data.Root != cliProj.Data.Root {
			t.Errorf("app %q root mismatch: static=%q, cli=%q", name, statProj.Data.Root, cliProj.Data.Root)
		}
		if _, ok := statProj.Data.Targets["build"]; !ok {
			t.Errorf("app %q missing build target in static metadata", name)
		}
	}
}
```

- [x] **Step 2: Refactor `ReadMetadata` in `internal/workspace/nx.go`**

Rename current `ReadMetadata` body to `readMetadataNxCli(ctx context.Context, root string) (Metadata, error)` and implement top-level `ReadMetadata`:
```go
func ReadMetadata(ctx context.Context, root string) (Metadata, error) {
	if m, err := ReadMetadataStatic(root); err == nil && len(m.Graph.Nodes) > 0 {
		return m, nil
	}
	return readMetadataNxCli(ctx, root)
}
```

- [x] **Step 3: Run all workspace tests**

Run: `rtk go test -v ./internal/workspace`
Expected: PASS

- [x] **Step 4: Commit Task 2**

```bash
rtk git add internal/workspace/nx.go internal/workspace/static_test.go
rtk git commit -m "feat(workspace): wire ReadMetadata to use fast-path static parser with CLI fallback"
```

---

### Task 3: Add Benchmarks and Standalone Workspaces Support Test

**Files:**
- Modify: `internal/workspace/nx_test.go`
- Modify: `docs/roadmap.md`

- [x] **Step 1: Write microbenchmark in `internal/workspace/nx_test.go`**

Add:
```go
func BenchmarkReadMetadataStatic(b *testing.B) {
	wd, err := os.Getwd()
	if err != nil {
		b.Fatal(err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	testWorkspace := filepath.Join(repoRoot, "testdata", "nx-workspace")

	if _, err := os.Stat(filepath.Join(testWorkspace, "nx.json")); os.IsNotExist(err) {
		b.Skip("testdata/nx-workspace not found")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m, err := ReadMetadataStatic(testWorkspace)
		if err != nil || len(m.Graph.Nodes) == 0 {
			b.Fatalf("failed static read: %v", err)
		}
	}
}
```

- [x] **Step 2: Run benchmark to verify sub-5ms performance**

Run: `rtk go test -bench=BenchmarkReadMetadataStatic -benchmem ./internal/workspace`
Expected: PASS, ~0.5ms - 2ms per op.

- [x] **Step 3: Update `docs/roadmap.md`**

Update Stage 2.3 status to Completed in `docs/roadmap.md`.

- [x] **Step 4: Run full test suite across entire project**

Run: `rtk go test ./...`
Expected: 100% PASS

- [x] **Step 5: Run linter**

Run: `rtk golangci-lint run ./...`
Expected: No issues found

- [x] **Step 6: Commit Task 3**

```bash
rtk git add internal/workspace/nx_test.go docs/roadmap.md
rtk git commit -m "perf(workspace): add static metadata microbenchmarks and update roadmap"
```
