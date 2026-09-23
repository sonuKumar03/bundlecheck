# Application Source Regression Attribution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Attribute non-package application source code growth and lazy-to-initial transitions to specific source components and directories in PR comparisons, eliminating generic `(unattributed)` findings.

**Architecture:** Extend `AnalysisResult` and comparison findings to record and attribute application source contributions (non-node_modules files) across initial and lazy chunks. When regressions occur in application code, `GenerateFindings` identifies the specific source components/directories responsible from the bundle graph and ranks them as first-class `Kind: "source"` findings with trace paths. Backward compatibility with older baseline JSON files is preserved.

**Tech Stack:** Go 1.22+, esbuild / Angular stats metadata, Cobra CLI, GitHub Actions composite action.

**Spec:** `docs/agents.md` & `SKILL.md` contracts for bundle comparison and findings attribution.

## Global Constraints

- Never break backward compatibility with existing baseline JSON files (version `1`).
- Keep total reconciliation intact: sum(attributed packages) + sum(attributed sources) + sum(unattributed) == positive regression delta.
- All Go tests (`go test ./...`) must pass with zero race conditions.
- No third-party dependencies; use standard library and existing internal packages.
- Always prefix CLI commands with `rtk` per project rules.

---

### Task 1: Core Data Models & Source Aggregation in `AnalysisResult`

**Files:**
- Modify: `internal/analysis/summary.go:17-40`
- Modify: `internal/analysis/inspect.go:20-80`
- Test: `internal/analysis/summary_test.go`

**Interfaces:**
- Consumes: `snapshot.BundleSnapshot`, `snapshot.BundleOutput`, `snapshot.Contribution`
- Produces: `analysis.SourceContribution`, `AnalysisResult.Sources []SourceContribution`

- [ ] **Step 1: Write the failing unit test for application source extraction**

In `internal/analysis/summary_test.go`:
```go
func TestAnalyze_ApplicationSources(t *testing.T) {
	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Outputs: []snapshot.BundleOutput{
			{
				Path:    "main.js",
				Bytes:   50000,
				Initial: true,
				Inputs: []snapshot.Contribution{
					{Input: "node_modules/@angular/core/fesm2022/core.mjs", Bytes: 30000},
					{Input: "projects/movies/src/app/pages/movie-detail-page/movie-detail-page.component.ts", Bytes: 12000},
					{Input: "projects/movies/src/app/pages/movie-detail-page/movie-detail-page.component.html", Bytes: 8000},
				},
			},
			{
				Path:    "chunk-lazy.js",
				Bytes:   10000,
				Initial: false,
				Inputs: []snapshot.Contribution{
					{Input: "projects/movies/src/app/pages/person-page/person.component.ts", Bytes: 10000},
				},
			},
		},
	}

	res, err := analysis.Analyze(snap)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(res.Sources) == 0 {
		t.Fatalf("expected application sources to be populated, got 0")
	}

	// Verify grouping by directory / component root
	foundMovieDetail := false
	for _, s := range res.Sources {
		if strings.Contains(s.Name, "movie-detail-page") {
			foundMovieDetail = true
			if s.InitialBytes != 20000 {
				t.Errorf("expected 20000 initial bytes for movie-detail-page, got %d", s.InitialBytes)
			}
		}
	}
	if !foundMovieDetail {
		t.Errorf("expected movie-detail-page in sources, got: %+v", res.Sources)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/analysis -run TestAnalyze_ApplicationSources`
Expected: FAIL with `res.Sources undefined`

- [ ] **Step 3: Implement `SourceContribution` and source grouping in `internal/analysis/summary.go`**

Define:
```go
type SourceContribution struct {
	Name         string `json:"name"`
	InitialBytes int64  `json:"initialBytes"`
	LazyBytes    int64  `json:"lazyBytes"`
	TotalBytes   int64  `json:"totalBytes"`
}
```
Add `Sources []SourceContribution` to `AnalysisResult`. In `Analyze()`, filter non-package inputs (`!IsPackage(input)`), aggregate them by logical component directory (e.g. `path.Dir(input)` or directory containing `.component.ts`), and attach sorted `Sources` to `AnalysisResult`.

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk go test ./internal/analysis -run TestAnalyze_ApplicationSources`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
rtk git add internal/analysis/
rtk git commit -m "feat(analysis): track application source contributions in AnalysisResult"
```

---

### Task 2: Attribute Application Source Regressions in `GenerateFindings`

**Files:**
- Modify: `internal/comparison/explain.go:30-134`
- Modify: `internal/comparison/comparison.go:32-50`
- Test: `internal/comparison/explain_test.go`

**Interfaces:**
- Consumes: `res.Summary`, `res.Packages`, `before.Sources`, `after.Sources`, `snap *snapshot.BundleSnapshot`
- Produces: `Finding{Kind: "source", Name: "...", DeltaBytes: ..., InitialDelta: ..., TracePath: ...}`

- [ ] **Step 1: Write the failing unit test for source regression attribution**

In `internal/comparison/explain_test.go`:
```go
func TestGenerateFindings_AttributedSources(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary: snapshot.Totals{InitialJS: 100000, TotalJS: 100000},
		Packages: []snapshot.Package{},
		Sources: []analysis.SourceContribution{
			{Name: "projects/movies/src/app/pages/movie-detail-page", InitialBytes: 0, LazyBytes: 20000, TotalBytes: 20000},
		},
	}
	after := &analysis.AnalysisResult{
		Summary: snapshot.Totals{InitialJS: 120000, TotalJS: 100000},
		Packages: []snapshot.Package{},
		Sources: []analysis.SourceContribution{
			{Name: "projects/movies/src/app/pages/movie-detail-page", InitialBytes: 20000, LazyBytes: 0, TotalBytes: 20000},
		},
	}
	snap := &snapshot.BundleSnapshot{
		Outputs: []snapshot.BundleOutput{
			{
				Path:    "main.js",
				Initial: true,
				Inputs: []snapshot.Contribution{
					{Input: "projects/movies/src/app/pages/movie-detail-page/movie-detail-page.component.ts", Bytes: 20000},
				},
			},
		},
	}

	res := comparison.Compare(before, after)
	findings := comparison.GenerateFindings(res, snap)

	if len(findings) == 0 {
		t.Fatalf("expected findings, got 0")
	}

	var sourceFinding *comparison.Finding
	for i := range findings {
		if findings[i].Kind == "source" {
			sourceFinding = &findings[i]
			break
		}
	}
	if sourceFinding == nil {
		t.Fatalf("expected finding with Kind='source', got: %+v", findings)
	}
	if !strings.Contains(sourceFinding.Name, "movie-detail-page") {
		t.Errorf("expected movie-detail-page in finding name, got %q", sourceFinding.Name)
	}
	if sourceFinding.DeltaBytes != 20000 {
		t.Errorf("expected 20000 delta bytes, got %d", sourceFinding.DeltaBytes)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/comparison -run TestGenerateFindings_AttributedSources`
Expected: FAIL with `expected finding with Kind='source'` (currently returns `Kind='unattributed'`)

- [ ] **Step 3: Implement source finding attribution in `internal/comparison/explain.go`**

In `GenerateFindings(res *Result, snap *snapshot.BundleSnapshot)`:
1. When `effectiveTotal > attributedDelta`, inspect source deltas between `before.Sources` and `after.Sources` (or if `before.Sources` is nil, inspect application inputs in initial chunks of `snap`).
2. Rank positive source changes by byte delta.
3. For each source contributor that grew:
   - Create `Finding` with `Kind: "source"`, `Name: source.Name`, `DeltaBytes: effectiveDelta`, `InitialDelta: ...`.
   - Find emitted chunks for the source files.
   - Trace import path from entry point using `g.TracePackage` / `g.TracePath`.
   - Add to `attributedDelta`.
4. Only remaining positive byte delta (if any) is assigned to `(unattributed)`.
5. Ensure reconciliation invariant: `sum(attributed) + sum(unattributed) == effectiveTotal`.

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk go test ./internal/comparison -run TestGenerateFindings_AttributedSources`
Expected: PASS

- [ ] **Step 5: Run all comparison tests**

Run: `rtk go test ./internal/comparison`
Expected: PASS all tests

- [ ] **Step 6: Commit**

```bash
rtk git add internal/comparison/
rtk git commit -m "feat(comparison): attribute application source regressions to specific components"
```

---

### Task 3: Render Source Findings in Text, Markdown, and PR Reports

**Files:**
- Modify: `internal/report/comparison.go:60-80`
- Modify: `internal/report/markdown.go:70-100`
- Modify: `internal/report/github_pr.go:82-100`
- Test: `internal/report/comparison_test.go`
- Test: `internal/report/github_pr_test.go`
- Test: `internal/report/markdown_test.go`

**Interfaces:**
- Consumes: `Finding{Kind: "source", ...}`
- Produces: Formatted output showing source component indicators (e.g. `📁 [source]` or distinct markdown badge) and import paths

- [ ] **Step 1: Write failing test verifying source finding formatting in reports**

In `internal/report/github_pr_test.go`:
```go
func TestRenderPRComment_SourceFinding(t *testing.T) {
	res := &comparison.Result{
		Summary: comparison.SummaryChange{
			Delta: snapshot.Totals{InitialJS: 20480},
		},
		Findings: []comparison.Finding{
			{
				Name:       "projects/movies/src/app/pages/movie-detail-page",
				DeltaBytes: 20480,
				Kind:       "source",
				Chunks:     []string{"main.js"},
				Reason:     "Application component moved into initial bundle",
			},
		},
	}
	out := report.GitHubPRComment(res, nil, report.GitHubPROptions{})
	if !strings.Contains(out, "movie-detail-page") {
		t.Errorf("expected movie-detail-page in PR comment, got:\n%s", out)
	}
	if !strings.Contains(out, "📁") && !strings.Contains(out, "source") {
		t.Errorf("expected source indicator in PR comment, got:\n%s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/report -run TestRenderPRComment_SourceFinding`
Expected: FAIL

- [ ] **Step 3: Update `github_pr.go`, `markdown.go`, and `comparison.go` rendering**

Differentiate between `Kind == "package"`, `Kind == "source"`, and `Kind == "unattributed"`:
- Packages: `- 📦 **\`package-name\`** (\`+X KB\`) ...`
- Sources: `- 📁 **\`path/to/component\`** (\`+X KB\`) *(Application source moved to initial chunk)* ...`
- Unattributed: `- ⚪ **\`(unattributed)\`** (\`+X KB\`) ...`

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk go test ./internal/report -run TestRenderPRComment_SourceFinding`
Expected: PASS

- [ ] **Step 5: Run all report package tests**

Run: `rtk go test ./internal/report`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
rtk git add internal/report/
rtk git commit -m "feat(report): format application source regression findings in PR and markdown reports"
```

---

### Task 4: Full Test Suite, Release Bump to `v0.4.2`, and Live PR Verification

**Files:**
- Bump: `VERSION`, `npm/bundleradar/package.json`, docs via `./scripts/bump-version.sh 0.4.2`
- Test: All repository unit tests and contracts (`go test ./...`)

- [ ] **Step 1: Run complete repository test suite**

Run: `rtk go test ./...`
Expected: All tests pass

- [ ] **Step 2: Bump version to 0.4.2**

Run: `rtk ./scripts/bump-version.sh 0.4.2`
Expected: Clean version sync across all targets

- [ ] **Step 3: Build local binary and verify CLI output**

Run: `rtk go build -v -o bundleradar . && rtk ./bundleradar --version`
Expected: `bundleradar 0.4.2`

- [ ] **Step 4: Commit, tag, and push release**

```bash
rtk git commit -am "chore(release): bump version to 0.4.2 with source regression attribution"
rtk git push origin master
rtk git tag -fa v0.4.2 -m "Release v0.4.2"
rtk git tag -fa v0 -m "Release v0"
rtk git push origin v0.4.2
rtk git push origin v0 --force
rtk gh release create v0.4.2 --title "v0.4.2" --notes "### Features
- Direct application source attribution for regressions (replaces generic unattributed with actual component paths).
- Distinguish package vs. application component regressions in PR reports." --repo sonuKumar03/bundleradar
```

- [ ] **Step 5: Re-run PR #1 check on `angular-movies` to verify live output**

Run:
```bash
rtk gh run rerun 35542685777 --repo sonuKumar03/angular-movies
```
Watch the rerun and verify that the PR comment on `sonuKumar03/angular-movies#1` now explicitly displays:
`- 📁 **projects/movies/src/app/pages/movie-detail-page** (+20.2 KB)`
instead of `(unattributed)`!
