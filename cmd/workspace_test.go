package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bundlecheck/internal/workspace"
)

func TestWorkspaceSummaryWarnsAboutNewerBundleInput(t *testing.T) {
	root := nxFixture(t)
	stats := filepath.Join(root, "dist/apps/shop/stats.json")
	source := filepath.Join(root, "libs/ui/main.ts")
	if err := os.MkdirAll(filepath.Dir(source), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, nil, 0600); err != nil {
		t.Fatal(err)
	}
	artifactTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(stats, artifactTime, artifactTime); err != nil {
		t.Fatal(err)
	}
	inputTime := time.Now().Add(time.Hour)
	if err := os.Chtimes(source, inputTime, inputTime); err != nil {
		t.Fatal(err)
	}

	var out, stderr bytes.Buffer
	args := []string{"workspace", "summary", "--root", root, "--projects", "shop", "--format", "json"}
	if code := Execute(args, &out, &stderr); code != 0 {
		t.Fatalf("JSON report: %d %s", code, stderr.String())
	}
	var result workspace.Result
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	freshness := result.Apps[0].Freshness
	if freshness == nil || freshness.Status != "stale-suspected" || freshness.NewestInput != "libs/ui/main.ts" {
		t.Fatalf("freshness: %+v", freshness)
	}

	out.Reset()
	stderr.Reset()
	args[len(args)-1] = "text"
	if code := Execute(args, &out, &stderr); code != 0 || !strings.Contains(out.String(), "artifacts may be stale") || !strings.Contains(out.String(), "libs/ui/main.ts") {
		t.Fatalf("text report: %d %s %s", code, out.String(), stderr.String())
	}
}

func nxFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node required for Nx CLI invocation check")
	}
	root := t.TempDir()
	files := map[string]string{
		"node_modules/nx/package.json": `{"bin":{"nx":"bin/nx.js"}}`,
		"nx.json":                      "{}",
		"node_modules/nx/bin/nx.js":    `if(process.argv.slice(2).join(' ') !== 'graph --print' || process.env.NX_DAEMON !== 'false') process.exit(9); console.log(JSON.stringify({graph:{nodes:{shop:{type:'app',data:{root:'apps/shop',targets:{build:{executor:'@nx/angular:application',options:{outputPath:'dist/apps/shop'},configurations:{production:{}}}}}},admin:{type:'app',data:{root:'apps/admin',targets:{build:{executor:'@nx/angular:application',options:{outputPath:'dist/apps/admin'},configurations:{production:{}}}}}},ui:{type:'lib',data:{root:'libs/ui'}},legacy:{type:'app',data:{root:'apps/legacy',targets:{build:{executor:'@nx/angular:webpack-browser'}}}}}}}));`,
	}
	for name, data := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"shop", "admin"} {
		base := filepath.Join(root, "dist/apps", name)
		if err := os.MkdirAll(filepath.Join(base, "browser"), 0755); err != nil {
			t.Fatal(err)
		}
		for _, file := range []string{"stats.json", "browser/index.html", "browser/main.js"} {
			data, err := os.ReadFile(filepath.Join("..", "testdata/minimal", file))
			if err != nil {
				t.Fatal(err)
			}
			if file == "stats.json" {
				data = bytes.ReplaceAll(data, []byte("src/main.ts"), []byte("libs/ui/main.ts"))
			}
			if err := os.WriteFile(filepath.Join(base, file), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func TestWorkspaceSummary(t *testing.T) {
	root := nxFixture(t)
	run := func(extra ...string) (workspace.Result, int, string) {
		t.Helper()
		var out, errOut bytes.Buffer
		args := append([]string{"workspace", "summary", "--root", root, "--format", "json"}, extra...)
		code := Execute(args, &out, &errOut)
		var r workspace.Result
		if err := json.Unmarshal(out.Bytes(), &r); err != nil {
			t.Fatalf("JSON: %v stdout %s stderr %s", err, out.String(), errOut.String())
		}
		return r, code, errOut.String()
	}
	r, code, stderr := run()
	if code != 0 || stderr != "" || !r.Complete || len(r.Apps) != 3 || len(r.Findings) != 2 || len(r.Libraries) != 1 || r.Libraries[0].InitialBytes != 1600 {
		t.Fatalf("report: %+v exit %d %s", r, code, stderr)
	}
	if len(r.Apps[0].DrillDown) != 2 {
		t.Fatal("missing drill down commands")
	}
	for _, format := range []string{"text", "markdown"} {
		var out, errOut bytes.Buffer
		if Execute([]string{"workspace", "summary", "--root", root, "-f", format}, &out, &errOut) != 0 || !strings.Contains(out.String(), "lodash") || !strings.Contains(out.String(), "800 B") {
			t.Fatalf("%s: %s %s", format, out.String(), errOut.String())
		}
	}
	if err := os.Remove(filepath.Join(root, "dist/apps/admin/stats.json")); err != nil {
		t.Fatal(err)
	}
	r, code, _ = run()
	if code != 1 || r.Complete || r.Apps[0].Status != "missing-artifacts" || r.Packages[0].Apps["admin"] != nil || len(r.Findings) != 0 {
		t.Fatalf("partial: %+v exit %d", r, code)
	}
	r, code, _ = run("--projects", "shop")
	if code != 0 || !r.Complete || len(r.Apps) != 1 {
		t.Fatalf("selection: %+v exit %d", r, code)
	}
	r, code, _ = run("--projects", "legacy")
	if code != 1 || r.Complete {
		t.Fatalf("unsupported explicit: %+v exit %d", r, code)
	}
	r, code, _ = run("--projects", "unknown")
	if code != 1 || r.Apps[0].Status != "unknown-project" {
		t.Fatalf("unknown: %+v exit %d", r, code)
	}
	r, code, _ = run("--projects", "shop", "--configuration", "missing")
	if code != 1 || r.Apps[0].Status != "configuration-error" {
		t.Fatalf("configuration: %+v exit %d", r, code)
	}
}

func TestWorkspaceArtifactsAndMetadataErrors(t *testing.T) {
	root := nxFixture(t)
	cli := filepath.Join(root, "node_modules/nx/bin/nx.js")
	data, err := os.ReadFile(cli)
	if err != nil {
		t.Fatal(err)
	}
	// Nx's declared entry point may change across releases.
	moved := filepath.Join(root, "node_modules/nx/dist/bin/nx.js")
	if err := os.MkdirAll(filepath.Dir(moved), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(cli, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules/nx/package.json"), []byte(`{"bin":{"nx":"./dist/bin/nx.js"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	cli = moved
	run := func(extra ...string) (workspace.Result, int, string) {
		t.Helper()
		var out, stderr bytes.Buffer
		code := Execute(append([]string{"workspace", "summary", "--root", root, "--format", "json"}, extra...), &out, &stderr)
		var r workspace.Result
		if out.Len() > 0 {
			if err := json.Unmarshal(out.Bytes(), &r); err != nil {
				t.Fatalf("invalid partial JSON: %s", out.String())
			}
		}
		return r, code, stderr.String()
	}
	r, code, stderr := run()
	if code != 0 || !r.Complete {
		t.Fatalf("declared CLI path: %d %s", code, stderr)
	}
	if err := os.WriteFile(cli, bytes.ReplaceAll(data, []byte("dist/apps/admin"), []byte("dist/apps/shop")), 0600); err != nil {
		t.Fatal(err)
	}
	r, code, _ = run()
	if code != 1 || r.Apps[0].Status != "ambiguous-artifacts" || r.Apps[2].Status != "ambiguous-artifacts" || len(r.Packages) != 0 {
		t.Fatalf("duplicate ownership: %+v", r)
	}
	if err := os.WriteFile(cli, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dist/apps/shop/stats.json"), []byte("not JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	r, code, _ = run()
	if code != 1 || r.Apps[0].Status != "analyzed" || r.Apps[2].Status != "analysis-error" || r.Packages[0].Apps["shop"] != nil {
		t.Fatalf("analysis failure: %+v", r)
	}
	if err := os.WriteFile(cli, []byte(`console.log('not JSON');`), 0600); err != nil {
		t.Fatal(err)
	}
	_, code, stderr = run()
	if code != 1 || !strings.Contains(stderr, "invalid Nx graph JSON") {
		t.Fatalf("metadata failure: %d %s", code, stderr)
	}
}

func TestWorkspaceFileOutput(t *testing.T) {
	root := nxFixture(t)
	path := filepath.Join(t.TempDir(), "report.json")
	var out, stderr bytes.Buffer
	if code := Execute([]string{"workspace", "summary", "--root", root, "-f", "json", "-o", path}, &out, &stderr); code != 0 || out.Len() != 0 {
		t.Fatalf("file output: %d %s", code, stderr.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var r workspace.Result
	if err := json.Unmarshal(data, &r); err != nil || !r.Complete {
		t.Fatalf("file JSON: %v %+v", err, r)
	}
	for _, args := range [][]string{{"--format", "yaml"}, {"--top", "0"}, {"--projects", ""}, {"--target", ""}, {"--configuration", ""}} {
		out.Reset()
		stderr.Reset()
		if Execute(append([]string{"workspace", "summary", "--root", root}, args...), &out, &stderr) != 1 || out.Len() != 0 {
			t.Fatalf("invalid flags accepted: %v %s", args, out.String())
		}
	}
}
