package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundlecheck/internal/advisor"
	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/baseline"
	"github.com/sonuKumar03/bundlecheck/internal/comparison"
	"github.com/sonuKumar03/bundlecheck/internal/graph"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

// setupTwoEntryFixture creates a temporary two-entry build artifact fixture:
// Entry 1: src/main.ts -> browser/main.js (1500 B, imports browser/chunk.js and main-pkg)
// Entry 2: src/worker.ts -> browser/worker.js (800 B, imports worker-pkg)
// Chunk: browser/chunk.js (500 B)
func setupTwoEntryFixture(t *testing.T) (statsFile, distDir string) {
	t.Helper()
	dir := t.TempDir()
	distDir = filepath.Join(dir, "dist", "browser")
	if err := os.MkdirAll(distDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(distDir, "main.js"), []byte(strings.Repeat("M", 1500)), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(distDir, "chunk.js"), []byte(strings.Repeat("C", 500)), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(distDir, "worker.js"), []byte(strings.Repeat("W", 800)), 0644); err != nil {
		t.Fatal(err)
	}
	html := `<!DOCTYPE html><html><head><script type="module" src="main.js"></script></head><body></body></html>`
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte(html), 0644); err != nil {
		t.Fatal(err)
	}

	statsContent := `{
  "inputs": {
    "src/main.ts": {"bytes": 200, "imports": [{"path": "src/app/main-component.ts"}]},
    "src/app/main-component.ts": {"bytes": 300, "imports": [{"path": "node_modules/main-pkg/index.js"}]},
    "node_modules/main-pkg/index.js": {"bytes": 1000, "imports": []},
    "src/chunk.ts": {"bytes": 500, "imports": []},
    "src/worker.ts": {"bytes": 100, "imports": [{"path": "node_modules/worker-pkg/index.js"}]},
    "node_modules/worker-pkg/index.js": {"bytes": 700, "imports": []}
  },
  "outputs": {
    "browser/main.js": {
      "bytes": 1500,
      "entryPoint": "src/main.ts",
      "inputs": {
        "src/main.ts": {"bytesInOutput": 200},
        "src/app/main-component.ts": {"bytesInOutput": 300},
        "node_modules/main-pkg/index.js": {"bytesInOutput": 1000}
      },
      "imports": [{"path": "browser/chunk.js", "kind": "import-statement"}]
    },
    "browser/chunk.js": {
      "bytes": 500,
      "inputs": {
        "src/chunk.ts": {"bytesInOutput": 500}
      },
      "imports": []
    },
    "browser/worker.js": {
      "bytes": 800,
      "entryPoint": "src/worker.ts",
      "inputs": {
        "src/worker.ts": {"bytesInOutput": 100},
        "node_modules/worker-pkg/index.js": {"bytesInOutput": 700}
      },
      "imports": []
    }
  }
}`
	statsFile = filepath.Join(dir, "stats.json")
	if err := os.WriteFile(statsFile, []byte(statsContent), 0644); err != nil {
		t.Fatal(err)
	}
	return statsFile, distDir
}

func TestEntryFlagRegistration(t *testing.T) {
	root := NewRootCommand()

	findSubcommand := func(names ...string) *cobra.Command {
		curr := root
		for _, name := range names {
			found, _, err := curr.Find([]string{name})
			if err != nil || found == nil {
				t.Fatalf("subcommand %q not found under %q", name, curr.Name())
			}
			curr = found
		}
		return curr
	}

	targets := []struct {
		path []string
	}{
		{[]string{"summary"}},
		{[]string{"suggest"}},
		{[]string{"why"}},
		{[]string{"measure"}},
		{[]string{"check"}},
		{[]string{"baseline", "save"}},
		{[]string{"baseline", "create"}},
	}

	const wantDesc = "Scope analysis to a specific entrypoint file or chunk name"
	for _, tt := range targets {
		name := strings.Join(tt.path, " ")
		t.Run(name, func(t *testing.T) {
			cmd := findSubcommand(tt.path...)
			f := cmd.Flags().Lookup("entry")
			if f == nil {
				t.Fatalf("%s does not expose --entry flag", name)
			}
			if f.Shorthand != "e" {
				t.Errorf("%s --entry shorthand = %q, want %q", name, f.Shorthand, "e")
			}
			if f.Usage != wantDesc {
				t.Errorf("%s --entry usage = %q, want %q", name, f.Usage, wantDesc)
			}
		})
	}
}

func TestSummaryWithEntry_ScopesInitialBytes(t *testing.T) {
	statsFile, distDir := setupTwoEntryFixture(t)

	// 1. Scoped to worker: initial is worker (800), lazy is main+chunk (2000)
	var outWorker, errWorker bytes.Buffer
	code := Execute([]string{"summary", statsFile, distDir, "--entry", "src/worker.ts", "-f", "json"}, &outWorker, &errWorker)
	if code != 0 {
		t.Fatalf("summary worker failed: %s", errWorker.String())
	}
	var resWorker analysis.AnalysisResult
	if err := json.Unmarshal(outWorker.Bytes(), &resWorker); err != nil {
		t.Fatalf("unmarshal worker summary: %v", err)
	}
	if resWorker.Summary.InitialJS != 800 {
		t.Errorf("expected worker InitialJS 800, got %d", resWorker.Summary.InitialJS)
	}
	if resWorker.Summary.LazyJS != 2000 {
		t.Errorf("expected worker LazyJS 2000, got %d", resWorker.Summary.LazyJS)
	}
	if resWorker.Summary.TotalJS != 2800 {
		t.Errorf("expected worker TotalJS 2800, got %d", resWorker.Summary.TotalJS)
	}

	// 2. Scoped to main: initial is main+chunk (2000), lazy is worker (800)
	var outMain, errMain bytes.Buffer
	code = Execute([]string{"summary", statsFile, distDir, "-e", "src/main.ts", "-f", "json"}, &outMain, &errMain)
	if code != 0 {
		t.Fatalf("summary main failed: %s", errMain.String())
	}
	var resMain analysis.AnalysisResult
	if err := json.Unmarshal(outMain.Bytes(), &resMain); err != nil {
		t.Fatalf("unmarshal main summary: %v", err)
	}
	if resMain.Summary.InitialJS != 2000 {
		t.Errorf("expected main InitialJS 2000, got %d", resMain.Summary.InitialJS)
	}
	if resMain.Summary.LazyJS != 800 {
		t.Errorf("expected main LazyJS 800, got %d", resMain.Summary.LazyJS)
	}
	if resMain.Summary.TotalJS != 2800 {
		t.Errorf("expected main TotalJS 2800, got %d", resMain.Summary.TotalJS)
	}

	// 3. Emitted glob matches worker
	var outGlob, errGlob bytes.Buffer
	code = Execute([]string{"summary", statsFile, distDir, "--entry", "*worker.js", "-f", "json"}, &outGlob, &errGlob)
	if code != 0 {
		t.Fatalf("summary glob failed: %s", errGlob.String())
	}
	var resGlob analysis.AnalysisResult
	if err := json.Unmarshal(outGlob.Bytes(), &resGlob); err != nil {
		t.Fatalf("unmarshal glob summary: %v", err)
	}
	if resGlob.Summary.InitialJS != 800 {
		t.Errorf("expected glob InitialJS 800, got %d", resGlob.Summary.InitialJS)
	}

	// 4. Invalid / unmatched entry returns error
	var outBad, errBad bytes.Buffer
	code = Execute([]string{"summary", statsFile, distDir, "--entry", "missing.ts"}, &outBad, &errBad)
	if code == 0 {
		t.Fatalf("expected error for unmatched entry, got code 0")
	}
	if !strings.Contains(errBad.String(), "missing.ts") {
		t.Errorf("expected error message to contain 'missing.ts', got: %s", errBad.String())
	}
}

func TestWhyWithEntry_ScopesRoot(t *testing.T) {
	statsFile, distDir := setupTwoEntryFixture(t)

	// 1. Tracing worker-pkg with --entry src/worker.ts: finds chain rooted at src/worker.ts
	var outWorker, errWorker bytes.Buffer
	code := Execute([]string{"why", "-s", statsFile, "-d", distDir, "worker-pkg", "--entry", "src/worker.ts", "-f", "json"}, &outWorker, &errWorker)
	if code != 0 {
		t.Fatalf("why worker failed: %s", errWorker.String())
	}
	var resWorker graph.WhyResult
	if err := json.Unmarshal(outWorker.Bytes(), &resWorker); err != nil {
		t.Fatalf("unmarshal why worker: %v", err)
	}
	if !resWorker.Found || len(resWorker.Chains) == 0 {
		t.Fatalf("expected worker-pkg to be found, got %+v", resWorker)
	}
	if len(resWorker.Chains[0].Path) == 0 || resWorker.Chains[0].Path[0] != "src/worker.ts" {
		t.Errorf("expected chain to root at src/worker.ts, got %v", resWorker.Chains[0].Path)
	}

	// 2. Tracing worker-pkg with --entry src/main.ts: not reachable from main
	var outMain, errMain bytes.Buffer
	code = Execute([]string{"why", "-s", statsFile, "-d", distDir, "worker-pkg", "--entry", "src/main.ts", "-f", "json"}, &outMain, &errMain)
	if code != 0 {
		t.Fatalf("why main failed: %s", errMain.String())
	}
	var resMain graph.WhyResult
	if err := json.Unmarshal(outMain.Bytes(), &resMain); err != nil {
		t.Fatalf("unmarshal why main: %v", err)
	}
	if len(resMain.Chains) != 0 {
		t.Errorf("expected 0 chains for worker-pkg when tracing from main.ts, got %d", len(resMain.Chains))
	}

	// 3. Tracing main-pkg with -e src/main.ts: finds chain rooted at src/main.ts
	var outMainPkg, errMainPkg bytes.Buffer
	code = Execute([]string{"why", "-s", statsFile, "-d", distDir, "main-pkg", "-e", "src/main.ts", "-f", "json"}, &outMainPkg, &errMainPkg)
	if code != 0 {
		t.Fatalf("why main-pkg failed: %s", errMainPkg.String())
	}
	var resMainPkg graph.WhyResult
	if err := json.Unmarshal(outMainPkg.Bytes(), &resMainPkg); err != nil {
		t.Fatalf("unmarshal why main-pkg: %v", err)
	}
	if !resMainPkg.Found || len(resMainPkg.Chains) == 0 {
		t.Fatalf("expected main-pkg to be found from main.ts, got %+v", resMainPkg)
	}
	if len(resMainPkg.Chains[0].Path) == 0 || resMainPkg.Chains[0].Path[0] != "src/main.ts" {
		t.Errorf("expected chain to root at src/main.ts, got %v", resMainPkg.Chains[0].Path)
	}

	// 4. Invalid entry errors
	var outBad, errBad bytes.Buffer
	code = Execute([]string{"why", "-s", statsFile, "-d", distDir, "worker-pkg", "--entry", "missing.ts"}, &outBad, &errBad)
	if code == 0 {
		t.Fatal("expected error for unmatched entry in why, got code 0")
	}
}

func TestSuggestWithEntry_ScopesImporter(t *testing.T) {
	statsFile, distDir := setupTwoEntryFixture(t)

	// 1. Scoped to worker: suggestion target is worker-pkg, importer is src/worker.ts
	var outWorker, errWorker bytes.Buffer
	code := Execute([]string{"suggest", statsFile, distDir, "--entry", "src/worker.ts", "-m", "500B", "-f", "json"}, &outWorker, &errWorker)
	if code != 0 {
		t.Fatalf("suggest worker failed: %s", errWorker.String())
	}
	var resWorker advisor.AdvisorResult
	if err := json.Unmarshal(outWorker.Bytes(), &resWorker); err != nil {
		t.Fatalf("unmarshal suggest worker: %v", err)
	}
	if len(resWorker.Suggestions) == 0 {
		t.Fatal("expected suggestions for worker")
	}
	foundWorkerPkg := false
	for _, s := range resWorker.Suggestions {
		if s.Target == "worker-pkg" {
			foundWorkerPkg = true
			if s.File != "src/worker.ts" {
				t.Errorf("expected importer src/worker.ts, got %q", s.File)
			}
		}
	}
	if !foundWorkerPkg {
		t.Errorf("expected worker-pkg in worker suggestions, got: %+v", resWorker.Suggestions)
	}

	// 2. Scoped to main: suggestion target is main-pkg, importer is src/app/main-component.ts
	var outMain, errMain bytes.Buffer
	code = Execute([]string{"suggest", statsFile, distDir, "-e", "src/main.ts", "-m", "500B", "-f", "json"}, &outMain, &errMain)
	if code != 0 {
		t.Fatalf("suggest main failed: %s", errMain.String())
	}
	var resMain advisor.AdvisorResult
	if err := json.Unmarshal(outMain.Bytes(), &resMain); err != nil {
		t.Fatalf("unmarshal suggest main: %v", err)
	}
	if len(resMain.Suggestions) == 0 {
		t.Fatal("expected suggestions for main")
	}
	foundMainPkg := false
	for _, s := range resMain.Suggestions {
		if s.Target == "main-pkg" {
			foundMainPkg = true
			if s.File != "src/app/main-component.ts" {
				t.Errorf("expected importer src/app/main-component.ts, got %q", s.File)
			}
		}
	}
	if !foundMainPkg {
		t.Errorf("expected main-pkg in main suggestions, got: %+v", resMain.Suggestions)
	}

	// 3. summary --suggest also scopes suggestions
	var outSum, errSum bytes.Buffer
	code = Execute([]string{"summary", statsFile, distDir, "--entry", "src/worker.ts", "--suggest", "-f", "json"}, &outSum, &errSum)
	if code != 0 {
		t.Fatalf("summary --suggest failed: %s", errSum.String())
	}
	var sumSuggest struct {
		analysis.AnalysisResult
		Suggestions []advisor.Suggestion `json:"suggestions"`
	}
	if err := json.Unmarshal(outSum.Bytes(), &sumSuggest); err != nil {
		t.Fatalf("unmarshal summary --suggest: %v", err)
	}
	for _, s := range sumSuggest.Suggestions {
		if s.Target == "worker-pkg" && s.File != "src/worker.ts" {
			t.Errorf("expected summary --suggest importer src/worker.ts, got %q", s.File)
		}
	}
}

func TestCheckWithEntry_ScopesBudget(t *testing.T) {
	statsFile, distDir := setupTwoEntryFixture(t)

	// 1. Scoped to worker: InitialJS is 800 B, passes --max-initial 1KB (1024 B)
	var outPass, errPass bytes.Buffer
	codePass := Execute([]string{"check", statsFile, distDir, "--entry", "src/worker.ts", "--max-initial", "1KB", "-f", "json"}, &outPass, &errPass)
	if codePass != ExitCodeSuccess {
		t.Fatalf("expected check to pass for worker, got code %d, stderr: %s", codePass, errPass.String())
	}

	// 2. Scoped to main: InitialJS is 2000 B, breaches --max-initial 1KB (1024 B)
	var outFail, errFail bytes.Buffer
	codeFail := Execute([]string{"check", statsFile, distDir, "-e", "src/main.ts", "--max-initial", "1KB", "-f", "json"}, &outFail, &errFail)
	if codeFail != ExitCodePolicyViolation {
		t.Fatalf("expected check to fail with PolicyViolation for main, got code %d, stderr: %s", codeFail, errFail.String())
	}
}

func TestMeasureWithEntry_ScopesCurrentBytes(t *testing.T) {
	statsFile, distDir := setupTwoEntryFixture(t)
	tmp := t.TempDir()
	basePath := filepath.Join(tmp, "baseline.json")

	baseRes := &analysis.AnalysisResult{
		SchemaVersion: "1",
		ToolVersion:   "0.4.2",
		Command:       "summary",
		Summary: snapshot.Totals{
			InitialJS: 1000,
			LazyJS:    1000,
			TotalJS:   2000,
		},
		Packages: []snapshot.Package{},
	}
	if err := baseline.Save(basePath, baseRes); err != nil {
		t.Fatal(err)
	}

	// 1. Measure scoped to worker: current InitialJS is 800 (delta: -200)
	var outWorker, errWorker bytes.Buffer
	code := Execute([]string{"measure", "-s", statsFile, "-d", distDir, "-b", basePath, "--entry", "src/worker.ts", "-f", "json"}, &outWorker, &errWorker)
	if code != 0 {
		t.Fatalf("measure worker failed: %s", errWorker.String())
	}
	var compWorker comparison.Result
	if err := json.Unmarshal(outWorker.Bytes(), &compWorker); err != nil {
		t.Fatalf("unmarshal measure worker: %v", err)
	}
	if compWorker.Summary.After.InitialJS != 800 {
		t.Errorf("expected Current InitialJS 800, got %d", compWorker.Summary.After.InitialJS)
	}
	if compWorker.Summary.Delta.InitialJS != -200 {
		t.Errorf("expected InitialDelta -200, got %d", compWorker.Summary.Delta.InitialJS)
	}

	// 2. Measure scoped to main: current InitialJS is 2000 (delta: +1000)
	var outMain, errMain bytes.Buffer
	code = Execute([]string{"measure", "-s", statsFile, "-d", distDir, "-b", basePath, "-e", "src/main.ts", "-f", "json"}, &outMain, &errMain)
	if code != 0 {
		t.Fatalf("measure main failed: %s", errMain.String())
	}
	var compMain comparison.Result
	if err := json.Unmarshal(outMain.Bytes(), &compMain); err != nil {
		t.Fatalf("unmarshal measure main: %v", err)
	}
	if compMain.Summary.After.InitialJS != 2000 {
		t.Errorf("expected Current InitialJS 2000, got %d", compMain.Summary.After.InitialJS)
	}
	if compMain.Summary.Delta.InitialJS != 1000 {
		t.Errorf("expected InitialDelta 1000, got %d", compMain.Summary.Delta.InitialJS)
	}
}

func TestBaselineSaveAndCreate_PersistsEntry(t *testing.T) {
	statsFile, distDir := setupTwoEntryFixture(t)
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpWd := t.TempDir()
	if err := os.Chdir(tmpWd); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	// 1. baseline save with --entry
	var outSave, errSave bytes.Buffer
	code := Execute([]string{"baseline", "save", "worker-base", "-s", statsFile, "-d", distDir, "--entry", "src/worker.ts"}, &outSave, &errSave)
	if code != 0 {
		t.Fatalf("baseline save failed: %s", errSave.String())
	}
	snapWorker, err := baseline.LoadSnapshot("worker-base")
	if err != nil {
		t.Fatalf("load worker-base: %v", err)
	}
	if snapWorker.Metadata == nil || snapWorker.Metadata.Entry != "src/worker.ts" {
		t.Errorf("expected saved metadata Entry 'src/worker.ts', got %+v", snapWorker.Metadata)
	}
	if snapWorker.Summary.InitialJS != 800 {
		t.Errorf("expected saved InitialJS 800, got %d", snapWorker.Summary.InitialJS)
	}

	// 2. baseline create with -e
	var outCreate, errCreate bytes.Buffer
	code = Execute([]string{"baseline", "create", "main-base", "-s", statsFile, "-d", distDir, "-e", "src/main.ts"}, &outCreate, &errCreate)
	if code != 0 {
		t.Fatalf("baseline create failed: %s", errCreate.String())
	}
	snapMain, err := baseline.LoadSnapshot("main-base")
	if err != nil {
		t.Fatalf("load main-base: %v", err)
	}
	if snapMain.Metadata == nil || snapMain.Metadata.Entry != "src/main.ts" {
		t.Errorf("expected created metadata Entry 'src/main.ts', got %+v", snapMain.Metadata)
	}
	if snapMain.Summary.InitialJS != 2000 {
		t.Errorf("expected created InitialJS 2000, got %d", snapMain.Summary.InitialJS)
	}
}

func TestBaselineRebuild_ReusesEntry(t *testing.T) {
	statsFile, distDir := setupTwoEntryFixture(t)
	defer func() {
		_ = baseline.Delete(baseline.DefaultDir, "rebuild-target")
	}()

	// Save an initial baseline linked to git ref HEAD with an Entry
	res := &analysis.AnalysisResult{
		SchemaVersion: "1",
		ToolVersion:   "0.4.2",
		Command:       "summary",
		Summary: snapshot.Totals{
			InitialJS: 999, // old value
			LazyJS:    999,
			TotalJS:   1998,
		},
		Packages: []snapshot.Package{},
	}
	meta := &baseline.Metadata{
		Name:      "rebuild-target",
		GitRef:    "HEAD",
		BuildCmd:  "true",
		Entry:     "src/worker.ts",
		CreatedAt: time.Now().UTC(),
	}
	_, err := baseline.SaveNamed(baseline.DefaultDir, "rebuild-target", res, meta)
	if err != nil {
		t.Fatalf("save initial baseline: %v", err)
	}

	// Rebuild the baseline
	var outRebuild, errRebuild bytes.Buffer
	code := Execute([]string{"baseline", "rebuild", "rebuild-target", "--no-build", "-s", statsFile, "-d", distDir}, &outRebuild, &errRebuild)
	if code != 0 {
		t.Fatalf("baseline rebuild failed: %s", errRebuild.String())
	}

	snapRebuilt, err := baseline.LoadSnapshot("rebuild-target")
	if err != nil {
		t.Fatalf("load rebuilt snapshot: %v", err)
	}
	if snapRebuilt.Metadata == nil || snapRebuilt.Metadata.Entry != "src/worker.ts" {
		t.Errorf("expected rebuilt metadata Entry 'src/worker.ts', got %+v", snapRebuilt.Metadata)
	}
	if snapRebuilt.Summary.InitialJS != 800 {
		t.Errorf("expected rebuilt InitialJS 800, got %d", snapRebuilt.Summary.InitialJS)
	}
}
