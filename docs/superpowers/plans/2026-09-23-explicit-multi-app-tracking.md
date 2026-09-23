# Explicit Multi-App Tracking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow consumers to explicitly track and enforce size budgets across multiple Angular applications in CI and CLI via `--projects` and `--app` flags without depending on `nx.json`, Nx CLI, or graph tools.

**Architecture:** Introduce a unified `AppTarget` resolver that parses `--projects` (via convention discovery) and `--app name=stats[:dist]` (via explicit paths). Decouple `internal/workspace/analyze.go` from `nx.json` so it can analyze any set of `AppTarget`s directly. Update `bundleradar check` to evaluate budgets across multiple apps and output a unified status table with policy exit codes. Update `action.yml` to support multi-project tracking with per-project baselines and a consolidated PR report. Fix `--project` resolution in `cmd/compare.go`.

**Tech Stack:** Go 1.23+, Cobra CLI, GitHub Actions Composite Action, Node.js (`actions/github-script`).

**Spec:** `docs/superpowers/specs/2026-09-23-explicit-multi-app-tracking-design.md`

## Global Constraints
- Always prefix all shell/terminal commands with `rtk ` when executing commands.
- Preserve 100% backward compatibility for existing Nx workspaces and single-project CLI invocations.
- Zero Node.js runtime requirement for static/convention multi-app tracking.
- Exit code contracts: 0 for success/pass, 1 for budget policy violations, 2 for flag/usage errors, 3 for runtime/IO failures.

---

### Task 1: Unified `AppTarget` Model and Target Parser

**Files:**
- Create: `internal/workspace/targets.go`
- Test: `internal/workspace/targets_test.go`

**Interfaces:**
- Produces:
  ```go
  package workspace

  type AppTarget struct {
      Name  string
      Stats string
      Dist  string
  }

  func ParseAppTargets(rootDir string, projectNames []string, appSpecs []string) ([]AppTarget, error)
  ```
- Consumes:
  - `discovery.Locate(rootDir, projectName)` from `github.com/sonuKumar03/bundleradar/internal/discovery`

- [ ] **Step 1: Write failing tests for `ParseAppTargets`**

Create `internal/workspace/targets_test.go` covering:
1. Parsing explicit `--app` with `name=stats:dist`.
2. Parsing explicit `--app` with `name=stats` (auto-resolving sibling `browser/` directory).
3. Parsing `--projects` using `discovery.Locate`.
4. Error on invalid `--app` syntax (missing `=`, missing stats file).
5. Error on non-existent stats file.
6. Error on duplicate project name.
7. Error on duplicate canonical stats file path between distinct project names.

```go
package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAppTargets_ExplicitApp(t *testing.T) {
	tmp := t.TempDir()
	stats1 := filepath.Join(tmp, "portal-stats.json")
	dist1 := filepath.Join(tmp, "portal-dist")
	if err := os.WriteFile(stats1, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dist1, 0755); err != nil {
		t.Fatal(err)
	}

	targets, err := ParseAppTargets(tmp, nil, []string{"portal=" + stats1 + ":" + dist1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Name != "portal" || targets[0].Stats != stats1 || targets[0].Dist != dist1 {
		t.Errorf("unexpected target: %+v", targets[0])
	}
}

func TestParseAppTargets_DuplicateNames(t *testing.T) {
	tmp := t.TempDir()
	stats1 := filepath.Join(tmp, "stats.json")
	_ = os.WriteFile(stats1, []byte("{}"), 0644)

	_, err := ParseAppTargets(tmp, nil, []string{"portal=" + stats1, "portal=" + stats1})
	if err == nil {
		t.Fatal("expected error on duplicate project name")
	}
}

func TestParseAppTargets_CollidingArtifacts(t *testing.T) {
	tmp := t.TempDir()
	stats1 := filepath.Join(tmp, "stats.json")
	_ = os.WriteFile(stats1, []byte("{}"), 0644)

	_, err := ParseAppTargets(tmp, nil, []string{"portal=" + stats1, "admin=" + stats1})
	if err == nil {
		t.Fatal("expected error on colliding artifact paths")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/workspace -run TestParseAppTargets`
Expected: FAIL (compilation error, `ParseAppTargets` not defined)

- [ ] **Step 3: Implement `ParseAppTargets`**

Create `internal/workspace/targets.go`:

```go
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/discovery"
)

// AppTarget represents an explicitly declared or discovered application bundle target.
type AppTarget struct {
	Name  string
	Stats string
	Dist  string
}

// ParseAppTargets parses and validates application targets from --projects and --app specs.
func ParseAppTargets(rootDir string, projectNames []string, appSpecs []string) ([]AppTarget, error) {
	if rootDir == "" {
		var err error
		rootDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve root dir: %w", err)
	}

	var targets []AppTarget
	seenNames := make(map[string]bool)
	seenStats := make(map[string]string)

	add := func(target AppTarget) error {
		canonicalName := strings.ToLower(strings.TrimSpace(target.Name))
		if canonicalName == "" {
			return fmt.Errorf("app target name cannot be empty")
		}
		if seenNames[canonicalName] {
			return fmt.Errorf("duplicate project name %q", target.Name)
		}

		cleanStats := target.Stats
		if !filepath.IsAbs(cleanStats) {
			cleanStats = filepath.Join(absRoot, cleanStats)
		}
		cleanStats = filepath.Clean(cleanStats)

		fi, err := os.Stat(cleanStats)
		if err != nil {
			return fmt.Errorf("stats file for %q not found: %w", target.Name, err)
		}
		if fi.IsDir() {
			return fmt.Errorf("stats path for %q is a directory, expected file: %s", target.Name, cleanStats)
		}

		canonicalStats, err := filepath.EvalSymlinks(cleanStats)
		if err != nil {
			canonicalStats = cleanStats
		}
		if owner, exists := seenStats[canonicalStats]; exists {
			return fmt.Errorf("stats file %q is claimed by multiple projects (%q and %q)", cleanStats, owner, target.Name)
		}

		cleanDist := target.Dist
		if cleanDist == "" {
			statsDir := filepath.Dir(cleanStats)
			browserDir := filepath.Join(statsDir, "browser")
			if bfi, err := os.Stat(browserDir); err == nil && bfi.IsDir() {
				cleanDist = browserDir
			} else {
				cleanDist = statsDir
			}
		} else if !filepath.IsAbs(cleanDist) {
			cleanDist = filepath.Join(absRoot, cleanDist)
		}
		cleanDist = filepath.Clean(cleanDist)

		target.Stats = cleanStats
		target.Dist = cleanDist
		targets = append(targets, target)
		seenNames[canonicalName] = true
		seenStats[canonicalStats] = target.Name
		return nil
	}

	// 1. Process explicit --app specs (name=stats[:dist])
	for _, spec := range appSpecs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		eqIdx := strings.Index(spec, "=")
		if eqIdx <= 0 {
			return nil, fmt.Errorf("invalid --app specification %q: expected format name=stats_path[:dist_path]", spec)
		}
		name := strings.TrimSpace(spec[:eqIdx])
		paths := strings.TrimSpace(spec[eqIdx+1:])
		var statsPath, distPath string
		colonIdx := strings.Index(paths, ":")
		if colonIdx > 0 {
			statsPath = paths[:colonIdx]
			distPath = paths[colonIdx+1:]
		} else {
			statsPath = paths
		}

		if err := add(AppTarget{Name: name, Stats: statsPath, Dist: distPath}); err != nil {
			return nil, err
		}
	}

	// 2. Process convention --projects
	for _, proj := range projectNames {
		proj = strings.TrimSpace(proj)
		if proj == "" {
			continue
		}
		statsPath, distPath, err := discovery.Locate(absRoot, proj)
		if err != nil {
			return nil, fmt.Errorf("locate project %q: %w", proj, err)
		}
		if err := add(AppTarget{Name: proj, Stats: statsPath, Dist: distPath}); err != nil {
			return nil, err
		}
	}

	slices.SortFunc(targets, func(a, b AppTarget) int {
		return strings.Compare(a.Name, b.Name)
	})

	return targets, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk go test ./internal/workspace -run TestParseAppTargets`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
rtk git add internal/workspace/targets.go internal/workspace/targets_test.go
rtk git commit -m "feat(workspace): implement ParseAppTargets for explicit multi-app tracking"
```

---

### Task 2: Decouple `workspace.Analyze` and Update `bundleradar workspace summary`

**Files:**
- Modify: `internal/workspace/analyze.go`
- Modify: `cmd/workspace.go`
- Test: `internal/workspace/analyze_test.go`
- Test: `cmd/workspace_test.go`

**Interfaces:**
- Updates `workspace.AnalyzeOptions`:
  ```go
  type AnalyzeOptions struct {
      Root            string
      ExplicitRoot    bool
      Target          string
      Configuration   string
      Projects        []string
      AppTargets      []AppTarget // New: when non-empty, bypasses Nx discovery
      WithCompression bool
      AllowNxFallback bool
  }
  ```

- [ ] **Step 1: Write failing test in `internal/workspace/analyze_test.go`**

Add test verifying `workspace.Analyze` works on a temporary directory with **no `nx.json`** when `AppTargets` are provided:

```go
package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyze_WithAppTargets_NoNxJson(t *testing.T) {
	tmp := t.TempDir()
	// Deliberately no nx.json!
	app1Dir := filepath.Join(tmp, "dist", "portal")
	app2Dir := filepath.Join(tmp, "dist", "admin")
	_ = os.MkdirAll(filepath.Join(app1Dir, "browser"), 0755)
	_ = os.MkdirAll(filepath.Join(app2Dir, "browser"), 0755)

	minimalStats, _ := os.ReadFile("../testdata/minimal/stats.json")
	_ = os.WriteFile(filepath.Join(app1Dir, "stats.json"), minimalStats, 0644)
	_ = os.WriteFile(filepath.Join(app2Dir, "stats.json"), minimalStats, 0644)
	_ = os.WriteFile(filepath.Join(app1Dir, "browser", "index.html"), []byte("<html></html>"), 0644)
	_ = os.WriteFile(filepath.Join(app2Dir, "browser", "index.html"), []byte("<html></html>"), 0644)

	targets := []AppTarget{
		{Name: "portal", Stats: filepath.Join(app1Dir, "stats.json"), Dist: filepath.Join(app1Dir, "browser")},
		{Name: "admin", Stats: filepath.Join(app2Dir, "stats.json"), Dist: filepath.Join(app2Dir, "browser")},
	}

	res, err := Analyze(context.Background(), AnalyzeOptions{
		Root:       tmp,
		AppTargets: targets,
	})
	if err != nil {
		t.Fatalf("Analyze with explicit targets failed without nx.json: %v", err)
	}
	if !res.Complete {
		t.Fatalf("expected complete report, got incomplete: %+v", res)
	}
	if len(res.Apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(res.Apps))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/workspace -run TestAnalyze_WithAppTargets_NoNxJson`
Expected: FAIL (errors because `nx.json` not found in `Root()`)

- [ ] **Step 3: Update `internal/workspace/analyze.go`**

Modify `internal/workspace/analyze.go` to handle `len(opts.AppTargets) > 0`:

```go
	// In Analyze(ctx context.Context, opts AnalyzeOptions) (*Result, error):
	if len(opts.AppTargets) > 0 {
		workspaceRoot := opts.Root
		if workspaceRoot == "" {
			workspaceRoot, _ = os.Getwd()
		}
		r := NewResult(workspaceRoot, opts.Target, opts.Configuration)
		for _, target := range opts.AppTargets {
			app := App{
				Name:   target.Name,
				Stats:  target.Stats,
				Dist:   target.Dist,
				Status: "ready",
			}
			r.Apps = append(r.Apps, app)
		}
		// Proceed directly to analyzing apps, computing Matrix, and Findings
...
```

And in `cmd/workspace.go`:
Add `--app` flag (`var apps []string`) and parse targets:
```go
	c.Flags().StringSliceVar(&apps, "app", nil, "Explicit app mapping: name=stats_path[:dist_path] (can be repeated)")
```
If `len(apps) > 0` or if no `nx.json` is found but `--projects` was passed, call `workspace.ParseAppTargets` and pass `AppTargets` to `workspace.Analyze`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `rtk go test ./internal/workspace ./cmd -run "TestAnalyze_WithAppTargets|TestWorkspaceSummary"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
rtk git add internal/workspace/analyze.go cmd/workspace.go internal/workspace/analyze_test.go
rtk git commit -m "feat(workspace): decouple workspace summary from nx.json via explicit targets"
```

---

### Task 3: Fix `cmd/compare.go` Directory Disambiguation with `--project`

**Files:**
- Modify: `cmd/compare.go`
- Test: `cmd/compare_test.go`

**Interfaces:**
- Modifies: `loadSnapshotOrBuildWithSnapshot(pathOrName string, project string) (*analysis.AnalysisResult, *snapshot.BundleSnapshot, error)`
- Modifies: `loadSnapshotOrBuild(pathOrName string, project string) (*analysis.AnalysisResult, error)`

- [ ] **Step 1: Write failing test in `cmd/compare_test.go`**

Add a test that verifies `bundleradar compare` against a multi-app directory passes when `--project` is supplied:

```go
func TestCompare_MultiAppDirectoryWithProjectFlag(t *testing.T) {
	tmp := t.TempDir()
	// Create two apps under dist: portal and admin
	portalDist := filepath.Join(tmp, "dist", "portal", "browser")
	adminDist := filepath.Join(tmp, "dist", "admin", "browser")
	_ = os.MkdirAll(portalDist, 0755)
	_ = os.MkdirAll(adminDist, 0755)

	statsData, _ := os.ReadFile("../testdata/minimal/stats.json")
	_ = os.WriteFile(filepath.Join(tmp, "dist", "portal", "stats.json"), statsData, 0644)
	_ = os.WriteFile(filepath.Join(tmp, "dist", "admin", "stats.json"), statsData, 0644)
	_ = os.WriteFile(filepath.Join(portalDist, "index.html"), []byte("<html></html>"), 0644)
	_ = os.WriteFile(filepath.Join(adminDist, "index.html"), []byte("<html></html>"), 0644)

	baseJSON := filepath.Join(tmp, "baseline.json")
	_ = os.WriteFile(baseJSON, []byte(`{"schemaVersion":"1","summary":{"initialJs":1000,"lazyJs":500,"totalJs":1500},"packages":[]}`), 0644)

	var out, errOut bytes.Buffer
	code := Execute([]string{"compare", "--before", baseJSON, "--after", tmp, "--project", "portal", "-f", "json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected compare to succeed with --project portal, got exit %d: %s", code, errOut.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./cmd -run TestCompare_MultiAppDirectoryWithProjectFlag`
Expected: FAIL ("multiple Angular build outputs found")

- [ ] **Step 3: Update `cmd/compare.go`**

Update `loadSnapshotOrBuildWithSnapshot` to accept `project string`:

```go
// cmd/compare.go:68
a, snap, err := loadSnapshotOrBuildWithSnapshot(after, project)
...
// cmd/compare.go:130
func loadSnapshotOrBuildWithSnapshot(pathOrName, project string) (*analysis.AnalysisResult, *snapshot.BundleSnapshot, error) {
...
	if fi.IsDir() {
		sFile, dDir, err := resolveBuildArtifactsInDir(pathOrName, "", "", project)
		if err != nil {
			return nil, nil, err
		}
		return runAnalysisWithOptions(sFile, dDir, true)
	}
...
```

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk go test ./cmd -run TestCompare_MultiAppDirectoryWithProjectFlag`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
rtk git add cmd/compare.go cmd/compare_test.go
rtk git commit -m "fix(compare): forward --project flag to directory resolution"
```

---

### Task 4: Multi-App Budget Enforcement in `bundleradar check`

**Files:**
- Modify: `cmd/check.go`
- Modify: `internal/report/budget.go`
- Test: `cmd/check_test.go`

**Interfaces:**
- Produces:
  ```go
  type MultiAppCheckResult struct {
      Passed   bool                          `json:"passed"`
      Projects map[string]budget.CheckResult `json:"projects"`
      Summary  []ProjectCheckSummary         `json:"summary"`
  }

  type ProjectCheckSummary struct {
      Name       string `json:"name"`
      Passed     bool   `json:"passed"`
      InitialJS  int64  `json:"initialJs"`
      TotalJS    int64  `json:"totalJs"`
      Violations int    `json:"violations"`
  }
  ```

- [ ] **Step 1: Write failing tests for multi-app `check`**

Add tests in `cmd/check_test.go`:
1. `bundleradar check --projects portal,admin --max-initial 1MB` where both pass (exit code 0).
2. `bundleradar check --app portal=... --app admin=... --max-initial 500B` where one fails (exit code 1).
3. Test JSON and Markdown output formats for multi-app check.

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./cmd -run TestCheck_MultiApp`
Expected: FAIL

- [ ] **Step 3: Implement multi-app logic in `cmd/check.go` and `internal/report/budget.go`**

In `cmd/check.go`:
1. Add flags:
   ```go
   var projects string
   var apps []string
   c.Flags().StringVar(&projects, "projects", "", "Comma-separated project names to check")
   c.Flags().StringSliceVar(&apps, "app", nil, "Explicit app target mapping name=stats[:dist]")
   ```
2. When `projects != "" || len(apps) > 0`:
   - Parse targets via `workspace.ParseAppTargets(".", strings.Split(projects, ","), apps)`.
   - Iterate over targets, running analysis and budget validation.
   - Aggregate into `MultiAppCheckResult`.
   - Render text, markdown, or JSON via `internal/report`.
   - Return `ExitCodePolicyViolation` if `!result.Passed`.

In `internal/report/budget.go`:
- Implement `MultiAppCheckReport(w io.Writer, res MultiAppCheckResult, markdown bool) error`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `rtk go test ./cmd -run TestCheck`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
rtk git add cmd/check.go internal/report/budget.go cmd/check_test.go
rtk git commit -m "feat(check): add multi-app budget enforcement via --projects and --app"
```

---

### Task 5: Multi-Project CI in `action.yml`

**Files:**
- Modify: `action.yml`
- Test: `cmd/contracts_test.go`

**Interfaces:**
- Inputs in `action.yml`:
  - `projects`: Comma-separated list of projects.
  - `apps`: Explicit app mappings.

- [ ] **Step 1: Add inputs and multi-project runner logic in `action.yml`**

1. Declare inputs:
   ```yaml
   projects:
     description: 'Comma-separated project names to track in CI'
     required: false
     default: ''
   apps:
     description: 'Explicit app targets in format "name=stats:dist"'
     required: false
     default: ''
   ```
2. In the runner shell step:
   - If `INPUT_PROJECTS` or `INPUT_APPS` is set:
     - Invoke `bundleradar check` with `--projects "$INPUT_PROJECTS"` or `--app ...`.
     - Manage per-project baseline downloading and uploading.
3. In the PR Comment script:
   - For multi-project mode, render a consolidated markdown summary with status badge per app.
   - Tag with `<!-- bundleradar-multi-project-report -->` to ensure in-place updates.

- [ ] **Step 2: Run documentation & contract tests**

Run: `rtk go test . -run TestDocumentationContract -count=1`
Expected: PASS

- [ ] **Step 3: Run full test suite across packages**

Run: `rtk go test ./...`
Expected: PASS (all tests pass)

- [ ] **Step 4: Commit**

```bash
rtk git add action.yml
rtk git commit -m "feat(action): support explicit multi-project tracking and consolidated PR reporting"
```

---

### Self-Review Checklist
1. **Spec Coverage**: All items in `docs/superpowers/specs/2026-09-23-explicit-multi-app-tracking-design.md` have corresponding tasks (AppTarget parser, analyze decoupling, compare fix, multi-app check, and action.yml).
2. **No Placeholders**: Exact signatures, types, test cases, and file paths provided.
3. **Type Consistency**: `AppTarget`, `MultiAppCheckResult`, and `ParseAppTargets` signatures match across all tasks.
